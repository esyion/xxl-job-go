package xxl

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"xxljob-go/pkg/logx"
)

// startEmbedServer 内嵌 HTTP 服务，对应 Java EmbedServer(Netty)。
// 路由：/beat /idleBeat /trigger(/run 兼容别名) /kill /log
func (e *Executor) startEmbedServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /beat", e.wrapBiz(func(body []byte) any { return e.beat() }))
	mux.HandleFunc("POST /idleBeat", e.wrapBiz(func(body []byte) any {
		var req IdleBeatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return Fail[string]("invalid request body.")
		}
		return e.idleBeat(&req)
	}))
	mux.HandleFunc("POST /trigger", e.wrapBiz(e.triggerHTTP))
	mux.HandleFunc("POST /run", e.wrapBiz(e.triggerHTTP)) // 兼容旧版 admin
	mux.HandleFunc("POST /kill", e.wrapBiz(func(body []byte) any {
		var req KillRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return Fail[string]("invalid request body.")
		}
		return e.kill(&req)
	}))
	mux.HandleFunc("POST /log", e.wrapBiz(func(body []byte) any {
		var req LogRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return Fail[string]("invalid request body.")
		}
		return e.log(&req)
	}))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	e.httpServer = srv

	ln, err := net.Listen("tcp", e.listenAddr())
	if err != nil {
		return err
	}
	e.httpListener = ln
	e.serverWG.Add(1)
	go func() {
		defer e.serverWG.Done()
		_ = srv.Serve(ln)
	}()
	logx.Infof(">>>>>>>>>>> xxl-job remoting server start success, address:%s", e.Address())
	return nil
}

// wrapBiz 统一鉴权(accessToken+appname)、请求体读取与 JSON 响应。
func (e *Executor) wrapBiz(biz func(body []byte) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		resp := func() any {
			accessToken := r.Header.Get(HeaderAccessToken)
			appname := r.Header.Get(HeaderAppname)
			if accessToken == "" || appname == "" {
				return Fail[string]("invalid request, accessToken or appname is empty.")
			}
			if accessToken != e.cfg.AccessToken || appname != e.cfg.Appname {
				return Fail[string]("invalid request, accessToken or appname invalid.")
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
			if err != nil {
				return Fail[string]("read request body error.")
			}
			return biz(body)
		}()
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (e *Executor) triggerHTTP(body []byte) any {
	var req TriggerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return Fail[string]("invalid request body.")
	}
	return e.trigger(&req)
}

func (e *Executor) listenAddr() string {
	if e.cfg.Address != "" {
		// 仅取端口用于监听
		if _, port, err := net.SplitHostPort(e.cfg.Address); err == nil {
			return ":" + port
		}
	}
	return ":" + strconv.Itoa(e.cfg.Port)
}

// localIP 获取本机内网 IP：优先标准私网段(10/172.16-31/192.168)网卡地址。
func localIP() string {
	ifaces, err := net.Interfaces()
	if err == nil {
		var fallback string
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ip4, ok := addr.(*net.IPNet)
				if !ok || ip4.IP.To4() == nil || ip4.IP.IsLoopback() {
					continue
				}
				if isPrivateRFC1918(ip4.IP) {
					return ip4.IP.String()
				}
				if fallback == "" {
					fallback = ip4.IP.String()
				}
			}
		}
		if fallback != "" {
			return fallback
		}
	}
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func isPrivateRFC1918(ip net.IP) bool {
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"} {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
