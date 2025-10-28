package config

import (
	"fmt"
	"time"
	"project/global"
	"project/log"

	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

// 设置redis表的key
const (
	// redis的用户表名
	RedisKeyUsernames = "game:usernames"
	//这里是redis中各个游戏的表名
	RedisKeyTop10Best       = "game:guess:top10:best"  // 猜数字的游戏排行榜
	RedisKeyTop10FastestMap = "game:map:top10:fastest" // 地图游戏排行榜（用时最短）
	Cache_RateKey = "rmb_top10:cny"
)
const (
	CacheTTL     = 120 * time.Minute // 缓存时间
	LockTTL       = 10 * time.Second  
	WaitWarmup    = 5 * time.Second 
	PollInterval  = 120 * time.Millisecond
	Datasaved_TTL = 12*time.Hour
)
func initRedis() {
	RedisClient := redis.NewClient(&redis.Options{ //配置选项Options是结构体
		Addr:     AppConfig.Redis.Addr,
		DB:       AppConfig.Redis.DB,
		Password: AppConfig.Redis.Password,
		DialTimeout:  2 * time.Second,
        ReadTimeout:  800 * time.Millisecond,  // 读超时
        WriteTimeout: 800 * time.Millisecond,  // 写超时
        PoolSize:     20,
        MinIdleConns: 5,
	}) //返回一个客户端
	_, err := RedisClient.Ping().Result()
	if err != nil {
		log.L().Error("DataBase connection failed ,got error:", zap.Error(err))

	}
	global.RedisDB = RedisClient
	fmt.Println("2. Redis DataBase connection success!")
}
