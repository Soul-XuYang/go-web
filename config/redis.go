package config

import (
	"fmt"
	"project/global"
	"project/log"

	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

// 设置redis表的key
const (
	RedisKeyUsernames = "game:usernames"
	//这里是redis中各个游戏的表名
	RedisKeyTop10Best = "game:guess:top10:best"
)

func initRedis() {
	RedisClient := redis.NewClient(&redis.Options{ //配置选项Options是结构体
		Addr:     AppConfig.Redis.Addr,
		DB:       AppConfig.Redis.DB,
		Password: AppConfig.Redis.Password,
	}) //返回一个客户端
	_, err := RedisClient.Ping().Result()
	if err != nil {
		log.L().Error("DataBase connection failed ,got error:", zap.Error(err))

	}
	global.RedisDB = RedisClient
	fmt.Println("2. Redis DataBase connection success!")
}
