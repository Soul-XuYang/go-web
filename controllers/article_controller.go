package controllers

import (
	"net/http"
	"project/global"
	"project/models"

	"github.com/gin-gonic/gin"
)

func CreateArticle(c *gin.Context) {
	var article models.Article
	if err := c.ShouldBindJSON(&article); err != nil { //导入数据
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}
	if err := global.DB.Create(&article).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	c.JSON(200, article)
}
func GetArticles(c *gin.Context) {
	var articles []models.Article
	if err := global.DB.Find(&articles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	c.JSON(200, articles)
}
