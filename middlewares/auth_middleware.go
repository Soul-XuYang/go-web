package middlewares

import (
	"net/http"
	"project/global"
	"project/models"
	"project/utils"

	"github.com/gin-gonic/gin"
)

// 自定义中间件
func AuthMiddleWare() gin.HandlerFunc { //返回的是gin下的函数类型
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization") // 这里的键是Authorization
		if token == "" {
			if ck, err := c.Cookie(utils.CookieName); err == nil {
				token = ck
			}
		}
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort() //不中止
			return
		}
		username, err := utils.ParseJWT(token)
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
		c.Set("my_blog",models.My_blog_url)  // 这里先设定在我的文章
		c.Next() //调用下列的函数
	}
}
