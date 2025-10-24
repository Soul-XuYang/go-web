package controllers
import(
    "fmt"
    "context"
	"encoding/json"
    "github.com/go-redis/redis"
    "time"

    "project/global"
    "project/config"
)

// 创建缓存-存的是JSON数据
func setCache(ctx context.Context, key string,cache *rmbTop10Cache) error {
	b, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return global.RedisDB.Set(key, b, config.CacheTTL).Err()
}

func getCache(ctx context.Context,key string) (*rmbTop10Cache, error) { //这个就是按照已经设置的key获取数据
	val, err := global.RedisDB.Get(key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, err
		}
		return nil, fmt.Errorf(" redis get error: %w", err)
	}
	var cache rmbTop10Cache  //按照所需获取缓存的数据
	if err := json.Unmarshal(val, &cache); err != nil {
		return nil, fmt.Errorf(" cache decode error: %w", err)
	}
	return &cache, nil // 返回缓存的数据指针
}

func acquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return global.RedisDB.SetNX(key, "1", ttl).Result()
}

func releaseLock(ctx context.Context, key string) {
	// 忽略错误
	_ = global.RedisDB.Del(key).Err() // 删除这个键值对表
}