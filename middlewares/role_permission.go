package middlewares

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 新建一个角色权限管理-这里函数输入是可变参数-可以传入多个角色
func RolePermission(role_input ...string) gin.HandlerFunc { //双重校验-JWT和数据库
	return func(c *gin.Context) {
		// token := c.GetHeader("Authorization") // 这里的键是Authorization
		// if token == "" {
		// 	if ck, err := c.Cookie(utils.CookieName); err == nil {
		// 		token = ck
		// 	}
		// }
		// if token == "" {
		// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		// 	c.Abort() //中止
		// 	return
		// }
		// username, role, expireTime,err := utils.ParseJWT(token) //不管什么用户我都让其通过-获取role角色
		// if err != nil {
		// 	c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		// 	c.Abort()
		// 	return
		// }
		// 遍历可变参数，只要匹配任意一个角色就允许
		role := c.GetString("role")
		if role == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: role not found in context"})
			c.Abort()
			return
		}
		hasPermission := false
		for _, allowedRole := range role_input {
			if allowedRole == role {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{"error": "The user's role permission denied"})
			c.Abort()
			return
		}
		if exp, exists := c.Get("exp"); exists {
			if time.Now().Unix() > exp.(int64) { //.unix是unix时间戳表示的时间数字
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired,you need to login again"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// 超级管理员权限
func SuperAdminPermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: role not found in context"})
			c.Abort()
			return
		}
		if role != "superadmin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "The user's role permission denied"})
			c.Abort()
			return
		}
		// 可选：添加额外的安全检查
		// 检查token是否在有效期内
		if exp, exists := c.Get("exp"); exists {
			if time.Now().Unix() > exp.(int64) { //.unix是unix时间戳表示的时间数字
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired,you need to login again"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
