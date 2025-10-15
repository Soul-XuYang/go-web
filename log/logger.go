package log

import (
	"os"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	
)

const Version string = "0.0.1"

var logger *zap.Logger //建立一个公共的登录指针

func Init(flag bool) error {
	// 基础配置（沿用 dev/prod 的区别）
	var base zap.Config
	if flag {
		base = zap.NewProductionConfig()
	} else {
		base = zap.NewDevelopmentConfig()
		base.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	// ——时间与级别的统一格式——
	enc := base.EncoderConfig
	enc.TimeKey = "timestamp"
	enc.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05") // 去掉 .毫秒 与 +0800
	enc.EncodeLevel = zapcore.CapitalLevelEncoder                       // 无颜色

	// 普通日志（< ERROR）：不输出 caller
	encNoCaller := enc
	encNoCaller.CallerKey = "" // 这一行关键：没有 caller 字段

	// 错误日志（>= ERROR）：输出 caller
	encWithCaller := enc
	encWithCaller.CallerKey = "caller"

	var (
		encA zapcore.Encoder
		encB zapcore.Encoder
	)
	if flag {
		encA = zapcore.NewJSONEncoder(encNoCaller)
		encB = zapcore.NewJSONEncoder(encWithCaller)
	} else {
		encA = zapcore.NewConsoleEncoder(encNoCaller)
		encB = zapcore.NewConsoleEncoder(encWithCaller)
	}

	ws := zapcore.Lock(zapcore.AddSync(os.Stdout))

	// < ERROR 的日志（DEBUG/INFO/WARN）→ 不带 caller
	coreNoCaller := zapcore.NewCore(
		encA, ws,
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool { return lvl < zapcore.ErrorLevel }),
	)

	// ≥ ERROR 的日志（ERROR/DPANIC/PANIC/FATAL）→ 带 caller
	coreWithCaller := zapcore.NewCore(
		encB, ws,
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool { return lvl >= zapcore.ErrorLevel }),
	)

	l := zap.New( //报错返回的设置
		zapcore.NewTee(coreNoCaller, coreWithCaller),
		zap.AddCaller(), // 计算 caller，但只有 encWithCaller 才会编码输出
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel), // 可选：仅错误及以上打印之后数据的错误堆栈
	)

	logger = l //返回所悟对象
	return nil
}

// 延迟初始化和防御性编程-很巧妙的思维，防止其为初始化
func L() *zap.Logger {
	if logger == nil {
		_ = Init(false) // L() 做了个防御：如果发现还是 nil，就自动用开发配置初始化（Init(false)），至少保证不会空指针。
	}
	return logger
}
func Sync() { _ = L().Sync() } //确保所有日志都刷新到磁盘，保证缓存都除去了，错误忽略

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next() //继续后面的处理
		// 记录完整的日志信息
		L().Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("version", Version),
		)
	}
}
func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				L().Error("panic recovered",
					zap.Any("error", err),                  // 存入错误信息
					zap.ByteString("stack", debug.Stack()), // 记录堆栈信息
					zap.String("method", c.Request.Method), //对应的HTTP返回的方法
					zap.String("path", c.Request.URL.Path), //对应的HTTP返回的路径
				)
				c.AbortWithStatusJSON(500, gin.H{"error": "InternalServer error"}) // 返回 500（最简单版）
			}
		}()
		c.Next()
	}
}
