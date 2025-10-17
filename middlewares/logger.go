package middlewares

import (
	"project/log"
	"time"
	"github.com/gin-gonic/gin"
    "go.uber.org/zap"
    "project/config"
)

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next() //继续后面的处理
		// 记录完整的日志信息
		log.L().Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("version", config.Version),
		)
	}
}