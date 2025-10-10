package config

import (
	"fmt"
	"log"
	"project/global"

	"github.com/go-redis/redis"
)

func initRedis() {
	RedisClient := redis.NewClient(&redis.Options{ //配置选项Options是结构体
		Addr:     AppConfig.Redis.Addr,
		DB:       AppConfig.Redis.DB,
		Password: AppConfig.Redis.Password,
	}) //返回一个客户端
	_, err := RedisClient.Ping().Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis,got error:%v", err)

	}
	global.RedisDB = RedisClient
	fmt.Println("2. Redis DataBase connection success!")
}
