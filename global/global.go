package global

// 供后端代码的全局变量使用
import (
	"time"

	"github.com/go-redis/redis"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

var (
	DB           *gorm.DB // 数据库连接
	RedisDB      *redis.Client
	// 并行组
	FetchGroup   singleflight.Group
	// 单个请求的超时时间
	FetchTimeout = 3*time.Second 
)

