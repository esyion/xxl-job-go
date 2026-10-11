package xxl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// adminClient 执行器 -> 调度中心 的 HTTP 客户端。
type adminClient struct {
	address     string
	appname     string
	accessToken string
	httpClient  *http.Client
}

func (c *adminClient) post(uri string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(c.address, "/")+"/api/"+uri, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set(HeaderAppname, c.appname)
	req.Header.Set(HeaderAccessToken, c.accessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), nil
}

// startCallbackWorker 回调工作协程：攒批(最多50条/1秒)上报，失败落盘重试。
func (e *Executor) startCallbackWorker() {
	e.callbackWG.Add(2)

	// 1、回调队列消费
	go func() {
		defer e.callbackWG.Done()
		batch := make([]CallbackData, 0, 64)
		for {
			batch = batch[:0]
			select {
			case item := <-e.callbackChan:
				batch = append(batch, item)
				collect := true
				for collect && len(batch) < 50 {
					select {
					case item2 := <-e.callbackChan:
						batch = append(batch, item2)
					case <-time.After(time.Second):
						collect = false
					}
				}
				e.doCallback(batch)
			case <-e.callbackStop:
				// 退出前把队列里剩余的回调落盘
				for {
					select {
					case item := <-e.callbackChan:
						batch = append(batch, item)
					default:
						if len(batch) > 0 {
							e.persistCallbackFail(batch)
						}
						return
					}
				}
			}
		}
	}()

	// 2、失败重试：每 30s 扫描落盘文件
	go func() {
		defer e.callbackWG.Done()
		ticker := time.NewTicker(BeatTimeoutSec * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.retryCallbackFiles()
			case <-e.callbackStop:
				return
			}
		}
	}()
}

func (e *Executor) doCallback(list []CallbackData) {
	for _, client := range e.adminClients {
		body, err := client.post("callback", CallbackRequest{CallbackList: list})
		if err != nil {
			exLog.Printf("xxl-job callback error, address:%s, err:%v", client.address, err)
			continue
		}
		if parseCode(body) == CodeSuccess {
			return
		}
		exLog.Printf("xxl-job callback fail, address:%s, resp:%s", client.address, body)
	}
	e.persistCallbackFail(list)
}

// persistCallbackFail 回调失败落盘，等待重试线程补偿。
func (e *Executor) persistCallbackFail(list []CallbackData) {
	dir := e.appender.CallbackLogPath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		exLog.Printf("xxl-job persist callback fail-file error: %v", err)
		return
	}
	data, _ := json.Marshal(list)
	file := filepath.Join(dir, fmt.Sprintf("%d.callback", time.Now().UnixNano()))
	if err := os.WriteFile(file, data, 0644); err != nil {
		exLog.Printf("xxl-job persist callback fail-file error: %v", err)
	}
}

func (e *Executor) retryCallbackFiles() {
	dir := e.appender.CallbackLogPath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".callback") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		full := filepath.Join(dir, name)
		data, err := os.ReadFile(full)
		_ = os.Remove(full)
		if err != nil || len(data) == 0 {
			continue
		}
		var list []CallbackData
		if err := json.Unmarshal(data, &list); err != nil {
			continue
		}
		e.doCallback(list)
	}
}
