package middlewares

import (
	"project/config"
	"project/log"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GinLogger() gin.HandlerFunc { // 返回对应中间件的函数
	return func(c *gin.Context) {
		start := time.Now()
		c.Next() //继续后面的处理
		var errMsg string
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}
		// c.Writer 是 gin.Context 结构体中的一个字段，它的类型是 gin.ResponseWriter
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.Int("response size", c.Writer.Size()), // 响应的大小
			zap.String("version", config.Version),
		}
		// 记录完整的日志信息-每一次链接打印一边信息
		if errMsg != "" {
			fields = append(fields, zap.String("error", errMsg))
		}
		log.L().Info("HTTP Request", fields...) //期望的是多个字段参数
	}
}
