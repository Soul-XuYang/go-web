package config

import (
	"fmt"
	"project/global"
	"project/models"
	"sync"
	"time"

	"golang.org/x/time/rate"

	lru "github.com/hashicorp/golang-lru/v2" //本质上是双向链表+Hash表
)

var (
	// 全局LRU缓存实例
	LocalUserCache *lru.Cache[string, models.Users] //后续存的是一个结构体
	cacheOnce      sync.Once
	//  登录注册令牌限流器                       //确保其只执行一次即可-极其关键-确保初始化一次
	cleanupOnce   sync.Once
	LoginAttempts = sync.Map{}
)

func initUserCache(size int) { //size为全局变量
	cacheOnce.Do(func() {
		// 创建一个LRU缓存
		cache, err := lru.New[string, models.Users](size)
		if err != nil {
			panic(err)
		}
		LocalUserCache = cache
	})
}

func ClearUserCache(username string) {
	// 清理本地缓存
	cacheKey := fmt.Sprintf(RedisKeyUsers, username)
	LocalUserCache.Remove(cacheKey)
	// 清理Redis缓存
	global.RedisDB.Del(cacheKey)
}
func ensureCleanupRunning() {
	cleanupOnce.Do(func() {
		go cleanupOldLimiters()
	})
}
func cleanupOldLimiters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		LoginAttempts.Range(func(key, value interface{}) bool {
			limiter := value.(*rate.Limiter)

			// 直接检查并清理，不需要username
			if limiter.TokensAt(time.Now().Add(-5*time.Minute)) == float64(limiter.Burst()) {
				LoginAttempts.Delete(key)
			}
			return true
		})
	}
}
