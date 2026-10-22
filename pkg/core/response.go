package core

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应体。
//
// 对外只暴露 code / msg / request_id / data，绝不把内部错误字符串（SQL、strconv、JSON 解析细节）
// 返回给客户端；内部错误只写日志，靠 request_id 关联。
type Response struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	RequestID string      `json:"request_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// 中文错误文案表：对外统一用中文，避免中英混杂。
const (
	MsgBadRequest   = "请求参数不合法"
	MsgUnauthorized = "登录状态已失效，请重新登录"
	MsgForbidden    = "没有权限执行该操作"
	MsgNotFound     = "请求的资源不存在"
	MsgInternal     = "服务器内部错误，请稍后重试"
	MsgTooMany      = "请求过于频繁，请稍后再试"
)

func requestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString("request_id")
}

// Respond 统一响应入口。
// 400 及以上会按级别落日志：4xx 用 Warn，5xx 用 Error，并带上 request_id。
func Respond(c *gin.Context, code int, msg string, data interface{}, err error) {
	if msg == "" {
		msg = http.StatusText(code)
	}
	reqID := requestID(c)
	if err != nil {
		attrs := []any{
			"request_id", reqID,
			"status", code,
			"msg", msg,
			"error", err.Error(),
		}
		if code >= http.StatusInternalServerError {
			slog.Error("api error", attrs...)
		} else {
			slog.Warn("api rejected", attrs...)
		}
	}
	c.JSON(code, Response{
		Code:      code,
		Msg:       msg,
		RequestID: reqID,
		Data:      data,
	})
}

// Success 直接返回成功数据
func Success(c *gin.Context, data interface{}) {
	Respond(c, http.StatusOK, "success", data, nil)
}

// SuccessWithMessage 成功 + 自定义提示信息
func SuccessWithMessage(c *gin.Context, msg string, data interface{}) {
	Respond(c, http.StatusOK, msg, data, nil)
}

// Fail 自定义错误码 + 错误信息；err 只用于日志，不会序列化给客户端
func Fail(c *gin.Context, code int, msg string, err error) {
	Respond(c, code, msg, nil, err)
}

// BadRequest 参数错误（对外不含内部细节）
func BadRequest(c *gin.Context, msg string, err error) {
	if msg == "" {
		msg = MsgBadRequest
	}
	Respond(c, http.StatusBadRequest, msg, nil, err)
}

// 业务错误结构体（BizError)
type BizError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (c *BizError) Error() string {
	return c.Msg
}
func NewBizError(code int, msg string) *BizError {
	return &BizError{
		Code: code,
		Msg:  msg,
	}
}

// FailByError 自动判断错误类型：
//   - 业务错误（BizError）→ 按业务码返回，msg 是给用户看的中文提示
//   - 未知错误 → 统一 500，只返回通用文案，内部细节写日志
func FailByError(c *gin.Context, err error) {
	var bizError *BizError
	if errors.As(err, &bizError) {
		Respond(c, bizError.Code, bizError.Msg, nil, nil)
		return
	}
	Respond(c, http.StatusInternalServerError, MsgInternal, nil, err)
}
