package admin

import (
	"sort"
	"strings"
	"time"
)

// startRegistryWorker 注册维护协程：清死链 + 刷新自动注册执行器地址列表。
func (s *Service) startRegistryWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.refreshRegistry()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *Service) refreshRegistry() {
	deadBefore := time.Now().Add(-90 * time.Second)
	if _, err := s.store.RegistryRemoveDead(deadBefore); err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job registry remove dead error:%v", err)
		return
	}
	list, err := s.store.RegistryFindAll(deadBefore)
	if err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job registry find error:%v", err)
		return
	}
	// 按 appname 聚合地址
	groupAddress := map[string][]string{}
	for _, r := range list {
		if r.RegistryGroup != "EXECUTOR" {
			continue
		}
		groupAddress[r.RegistryKey] = append(groupAddress[r.RegistryKey], r.RegistryValue)
	}
	for key, addrs := range groupAddress {
		sort.Strings(addrs) // 稳定顺序，避免无谓刷新
		if err := s.store.UpdateGroupAddressListByAuto(key, strings.Join(addrs, ",")); err != nil {
			adLog.Printf(">>>>>>>>>>> xxl-job registry group address update error:%v", err)
		}
	}
}
