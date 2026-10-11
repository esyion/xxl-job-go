package xxl

import "encoding/json"

// 与 xxl-tool Response 对齐的统一响应结构，JSON 字段为 code/msg/data。
const (
	CodeSuccess = 200
	CodeFail    = 500
	CodeTimeout = 502
)

type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func Success[T any](data T) Response[T] {
	return Response[T]{Code: CodeSuccess, Msg: "Success", Data: data}
}

func SuccessMsg[T any](msg string) Response[T] {
	return Response[T]{Code: CodeSuccess, Msg: msg}
}

func Fail[T any](msg string) Response[T] {
	return Response[T]{Code: CodeFail, Msg: msg}
}

func Of[T any](code int, msg string) Response[T] {
	return Response[T]{Code: code, Msg: msg}
}

func (r Response[T]) IsSuccess() bool { return r.Code == CodeSuccess }

// parseResponse 解析 admin/executor 通用响应，忽略 data 具体类型。
func parseCode(body []byte) int {
	var r struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(body, &r)
	return r.Code
}
