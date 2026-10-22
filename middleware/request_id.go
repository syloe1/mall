package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// 定义一个key带存取请求ID
const RequestIDKey = "request_id"

// 一次请求，一个 ID，全流程日志绑定，快速定位线上问题！
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		//先从请求头里拿X-Request-ID
		requestID := c.GetHeader("X-Request-ID")
		//如果没有，就生成一个新的
		if requestID == "" {
			requestID = newRequestID()
		}

		// 3. 把ID存到 Gin 上下文
		c.Set(RequestIDKey, requestID)
		// 4. 把ID返回给前端（响应头）
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	value, ok := c.Get(RequestIDKey)
	if !ok {
		return ""
	}
	//把value强制转换成字符串
	requestID, _ := value.(string)
	return requestID
}

// newRequestID () 生成强随机数
func newRequestID() string {
	var b [16]byte
	// 往数组里填充 16 字节的【强随机数】
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	//  把 16 字节二进制 → 转成 32 位十六进制字符串
	return hex.EncodeToString(b[:])
}
