package middlewares

import (
	"net/http"
	"project/global"
	"project/models"
	"project/utils"

	"github.com/gin-gonic/gin"
)

// 新建一个角色权限管理
func RolePermission(role_input ...string) gin.HandlerFunc {
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
		username, role, err := utils.ParseJWT(token) //不管什么用户我都让其通过
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		for _, role_input := range role_input {
			if role == role_input {
				var u models.Users //查询用Select
				if err := global.DB.Select("id", "username").
					Where("username = ?", username). //where限定条件
					First(&u).Error; err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
					c.Abort()
					return
				}
				c.Set("username", username)
				c.Set("user_id", u.ID)
				c.Set("role", role)
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "The user's role permission denied"})
		c.Abort() //中止
	}
}
