package xxl

import (
	"time"
)

// startRegistryWorker 注册协程：每 30s 向 admin 心跳注册，成功即停止本轮。
func (e *Executor) startRegistryWorker() {
	e.registryWG.Add(1)
	go func() {
		defer e.registryWG.Done()
		ticker := time.NewTicker(BeatTimeoutSec * time.Second)
		defer ticker.Stop()
		// 启动立即注册一次
		e.registryOnce("registry")
		for {
			select {
			case <-ticker.C:
				e.registryOnce("registry")
			case <-e.registryStop:
				e.registryOnce("registryRemove")
				return
			}
		}
	}()
}

func (e *Executor) registryOnce(uri string) {
	param := RegistryRequest{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.cfg.Appname,
		RegistryValue: e.Address(),
	}
	for _, client := range e.adminClients {
		body, err := client.post(uri, param)
		if err != nil {
			exLog.Printf("xxl-job %s error, address:%s, err:%v", uri, client.address, err)
			continue
		}
		if parseCode(body) == CodeSuccess {
			return
		}
		exLog.Printf("xxl-job %s fail, address:%s, resp:%s", uri, client.address, body)
	}
}
