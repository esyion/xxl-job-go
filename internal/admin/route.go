package admin

import (
	"math/rand"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"xxljob-go/pkg/xxl"
)

// 路由策略常量，对应 ExecutorRouteStrategyEnum。
const (
	RouteFirst            = "FIRST"
	RouteLast             = "LAST"
	RouteRound            = "ROUND"
	RouteRandom           = "RANDOM"
	RouteConsistentHash   = "CONSISTENT_HASH"
	RouteLeastFrequentlyUsed = "LEAST_FREQUENTLY_USED"
	RouteLeastRecentlyUsed   = "LEAST_RECENTLY_USED"
	RouteFailover         = "FAILOVER"
	RouteBusyover         = "BUSYOVER"
	RouteShardingBroadcast = "SHARDING_BROADCAST"
)

var (
	routeRoundCount atomic.Int64

	lfuMu    sync.Mutex
	lfuMap   = map[int]map[string]int{} // jobId -> address -> 命中次数
	lfuStamp int64

	lruMu  sync.Mutex
	lruMap = map[int][]string{} // jobId -> 按访问序排列的地址
)

func localIPDesc() string {
	conn, err := net.Dial("udp", "10.255.255.255:1")
	if err != nil {
		conn, err = net.Dial("udp", "8.8.8.8:53")
		if err != nil {
			return "127.0.0.1"
		}
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// route 路由入口：返回 Response.Data=选中的地址。
func route(strategy string, req *xxl.TriggerRequest, group *JobGroup, regList []string, client *executorClient) xxl.Response[string] {
	switch strategy {
	case RouteFirst:
		return xxl.Success[string](regList[0])
	case RouteLast:
		return xxl.Success[string](regList[len(regList)-1])
	case RouteRound:
		n := int(routeRoundCount.Add(1) % int64(len(regList)))
		return xxl.Success[string](regList[n])
	case RouteRandom:
		return xxl.Success[string](regList[rand.Intn(len(regList))])
	case RouteConsistentHash:
		return xxl.Success[string](consistentHashRoute(req.JobID, regList))
	case RouteLeastFrequentlyUsed:
		return xxl.Success[string](lfuRoute(req.JobID, regList))
	case RouteLeastRecentlyUsed:
		return xxl.Success[string](lruRoute(req.JobID, regList))
	case RouteFailover:
		return failoverRoute(req, group, regList, client)
	case RouteBusyover:
		return busyoverRoute(req, group, regList, client)
	default:
		return xxl.Fail[string]("route strategy not found: " + strategy)
	}
}

// consistentHashRoute 一致性哈希：每地址 100 个虚拟节点，按 jobId 落环。
func consistentHashRoute(jobID int, regList []string) string {
	type vnode struct {
		hash  uint64
		addr  string
	}
	var ring []vnode
	for _, addr := range regList {
		for i := 0; i < 100; i++ {
			ring = append(ring, vnode{hash: fnv64(md5Hex(addr + "#" + strconv.Itoa(i))), addr: addr})
		}
	}
	sort.Slice(ring, func(i, j int) bool { return ring[i].hash < ring[j].hash })
	target := fnv64(md5Hex(strconv.Itoa(jobID)))
	idx := sort.Search(len(ring), func(i int) bool { return ring[i].hash >= target })
	if idx == len(ring) {
		idx = 0
	}
	return ring[idx].addr
}

func fnv64(s string) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range s {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

// lfuRoute 最不经常使用：每分钟清零衰减，选命中次数最少的地址。
func lfuRoute(jobID int, regList []string) string {
	lfuMu.Lock()
	defer lfuMu.Unlock()
	if minute := time.Now().Unix() / 60; minute != lfuStamp {
		lfuStamp = minute
		lfuMap = map[int]map[string]int{}
	}
	hits, ok := lfuMap[jobID]
	if !ok {
		hits = map[string]int{}
		lfuMap[jobID] = hits
	}
	best, bestHits := "", int(1<<30)
	for _, addr := range regList {
		h := hits[addr]
		if h < bestHits {
			best, bestHits = addr, h
		}
	}
	hits[best]++
	return best
}

// lruRoute 最近最久未使用：选最久未访问的地址并移到队尾。
func lruRoute(jobID int, regList []string) string {
	lruMu.Lock()
	defer lruMu.Unlock()
	order, ok := lruMap[jobID]
	if !ok {
		order = append([]string{}, regList...)
		lruMap[jobID] = order
	}
	// 补齐新增地址、剔除失效地址
	inSet := map[string]bool{}
	for _, addr := range regList {
		inSet[addr] = true
	}
	var kept []string
	for _, addr := range order {
		if inSet[addr] {
			kept = append(kept, addr)
			delete(inSet, addr)
		}
	}
	for _, addr := range regList {
		if inSet[addr] {
			kept = append(kept, addr)
		}
	}
	if len(kept) == 0 {
		kept = append(kept, regList...)
	}
	best := kept[0]
	kept = append(kept[1:], best)
	lruMap[jobID] = kept
	return best
}

// failoverRoute 故障转移：逐个 beat，成功即用。
func failoverRoute(req *xxl.TriggerRequest, group *JobGroup, regList []string, client *executorClient) xxl.Response[string] {
	var msg string
	for _, addr := range regList {
		result := client.beat(addr, group.AppName, group.AccessToken)
		msg += fmtRouteMsg("心跳探测：", addr, result.Code, result.Msg)
		if result.IsSuccess() {
			return xxl.Response[string]{Code: result.Code, Msg: msg, Data: addr}
		}
	}
	return xxl.Fail[string](msg)
}

// busyoverRoute 忙碌转移：逐个 idleBeat，空闲即用。
func busyoverRoute(req *xxl.TriggerRequest, group *JobGroup, regList []string, client *executorClient) xxl.Response[string] {
	var msg string
	for _, addr := range regList {
		result := client.idleBeat(addr, group.AppName, group.AccessToken, req.JobID)
		msg += fmtRouteMsg("忙碌探测：", addr, result.Code, result.Msg)
		if result.IsSuccess() {
			return xxl.Response[string]{Code: result.Code, Msg: msg, Data: addr}
		}
	}
	return xxl.Fail[string](msg)
}

func fmtRouteMsg(prefix, addr string, code int, msg string) string {
	return prefix + "<br>address：" + addr + "<br>code：" + strconv.Itoa(code) + "<br>msg：" + msg + "<br><br>"
}
