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
	// redis的统一全局参数
	CacheTTL     = 60 * time.Minute // 缓存时间
	FetchTimeout = 3500 * time.Millisecond
	FetchGroup   singleflight.Group
	// 请求的超时时间
	Timeout = 4*time.Second
)
