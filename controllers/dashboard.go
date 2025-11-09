package controllers

import (
	"net/http"
	"github.com/gin-gonic/gin"

)
// 权限管理-只有管理员可以访问
func GetDashboardArticleData(c *gin.Context) {
	userID := c.GetUint("user_id")
    userRole := c.GetString("role")
	if userID == 0||userRole != "admin" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission, user does not log in"})
		return
	}
}