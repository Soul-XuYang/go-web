package middlewares

import (
	"net/http"
	"project/global"
	"project/models"
	"project/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// 自定义中间件
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
		username, _, err := utils.ParseJWT(token) //不管什么用户我都让其通过
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		var u models.Users //查询用Select
		if err := global.DB.Select("id", "username").
			Where("username = ?", username). //where限定条件
			First(&u).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			c.Abort()
			return
		}
		c.Set("user_id", u.ID)
		c.Set("username", username)
		c.Set("my_blog", models.My_blog_url) // 这里先设定在我的文章
		c.Next()                             //调用下列的函数
	}
}
