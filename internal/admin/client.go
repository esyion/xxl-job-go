package admin

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"xxljob-go/pkg/xxl"
)

// executorClient 调度中心 -> 执行器 的 HTTP 客户端。
type executorClient struct {
	httpClient *http.Client
}

func newExecutorClient() *executorClient {
	return &executorClient{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (c *executorClient) call(address, appname, accessToken, uri string, body any) xxl.Response[string] {
	data, err := json.Marshal(body)
	if err != nil {
		return xxl.Fail[string](err.Error())
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(address, "/")+uri, bytes.NewReader(data))
	if err != nil {
		return xxl.Fail[string](err.Error())
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set(xxl.HeaderAccessToken, accessToken)
	req.Header.Set(xxl.HeaderAppname, appname)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return xxl.Fail[string](fmt.Sprintf("connect executor fail: %v", err))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var out xxl.Response[string]
	if err := json.Unmarshal(raw, &out); err != nil {
		return xxl.Fail[string](fmt.Sprintf("invalid executor response: %.200s", raw))
	}
	return out
}

func (c *executorClient) beat(address, appname, token string) xxl.Response[string] {
	return c.call(address, appname, token, "/beat", map[string]any{})
}

func (c *executorClient) idleBeat(address, appname, token string, jobID int) xxl.Response[string] {
	return c.call(address, appname, token, "/idleBeat", &xxl.IdleBeatRequest{JobID: jobID})
}

func (c *executorClient) trigger(address, appname, token string, req *xxl.TriggerRequest) xxl.Response[string] {
	return c.call(address, appname, token, "/trigger", req)
}

func (c *executorClient) kill(address, appname, token string, jobID int) xxl.Response[string] {
	return c.call(address, appname, token, "/kill", &xxl.KillRequest{JobID: jobID})
}

func (c *executorClient) log(address, appname, token string, req *xxl.LogRequest) xxl.Response[xxl.LogData] {
	data, err := json.Marshal(req)
	if err != nil {
		return xxl.Fail[xxl.LogData](err.Error())
	}
	httpReq, err := http.NewRequest(http.MethodPost, strings.TrimRight(address, "/")+"/log", bytes.NewReader(data))
	if err != nil {
		return xxl.Fail[xxl.LogData](err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json;charset=UTF-8")
	httpReq.Header.Set(xxl.HeaderAccessToken, token)
	httpReq.Header.Set(xxl.HeaderAppname, appname)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return xxl.Fail[xxl.LogData](err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var out xxl.Response[xxl.LogData]
	if err := json.Unmarshal(raw, &out); err != nil {
		return xxl.Fail[xxl.LogData](fmt.Sprintf("invalid executor response: %.200s", raw))
	}
	return out
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
