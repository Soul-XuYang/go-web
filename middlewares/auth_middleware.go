package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
	"project/config"
	"project/global"
	"project/models"
	"project/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// 自定义中间件
// 并且这里于登录对应双重检验
func AuthMiddleWare() gin.HandlerFunc { //返回的是gin下的函数类型
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("Authorization")) // 这里的键是Authorization
		if token == "" {
			if ck, err := c.Cookie(utils.CookieName); err == nil {
				token = ck
			}
		}
		// 去掉 "Bearer " 前缀（如果存在）
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		// 一定要做兼容，如果前端传来的是"Bearer xxx"，则需要去掉"Bearer "我们取得只有JWT生成的Token
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort() //不中止
			return
		}
		username, role, expireTime, err := utils.ParseJWT(token) //不管什么用户我都让其通过
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		setContext(c, username, role, expireTime) //提前已经设置了
		cacheKey := fmt.Sprintf(config.RedisKeyUsers, username)
		var u models.Users
		//L1 本地LRU缓存
		if data, exists := config.LocalUserCache.Get(cacheKey); exists {
			c.Set("user_id", data.ID)
			c.Next()
			return
		}
		//L2 Redis缓存
		if data, err := global.RedisDB.Get(cacheKey).Result(); err == nil {
			if err := json.Unmarshal([]byte(data), &u); err == nil {
				// 更新本地缓存
				config.LocalUserCache.Add(cacheKey, u)
				c.Set("user_id", u.ID)
				c.Next()
				return
			}
		}
		// 查询数据库
		if err := global.DB.Select("id"). //只给这三组数据
									Where("username = ?", username). //where限定条件
									First(&u).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			c.Abort()
			return
		}
		// 认证成功后
		if userData, err := json.Marshal(u); err == nil {
			global.RedisDB.Set(cacheKey, userData, config.CacheTTL) //2h
		}
		config.LocalUserCache.Add(cacheKey, u) //添加设置
		c.Set("user_id", u.ID)
		c.Next()
	}
}
func setContext(c *gin.Context, username, role string, expireTime int64) {
    c.Set("username", username)
    c.Set("role", role)
    c.Set("exp", expireTime)
    c.Set("my_blog", models.My_blog_url)
}
