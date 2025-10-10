package global

// 供后端代码的全局变量使用
import (
	"github.com/go-redis/redis"
	"gorm.io/gorm"
)

var (
	DB *gorm.DB // 数据库连接
	RedisDB *redis.Client
)
