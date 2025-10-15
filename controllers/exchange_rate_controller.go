package controllers

import (
	"net/http"
	"project/global"
	"project/models"
	"time"

	"github.com/gin-gonic/gin"
)

// 汇率数据
func CreateExchangeRate(c *gin.Context) {
	var exchangeRate models.ExchangeRate
	// 请求体里是否有
	if err := c.ShouldBindJSON(&exchangeRate); err != nil { //导入请求体数据
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	exchangeRate.Date = time.Now() //当前时间
	//插入数据
	if err := global.DB.Create(&exchangeRate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

// 可以用这种方式查看表
// fmt.Println(global.DB.Model(&ExchangeRate{}).Statement.Table) // 输出: exchange_rates
// fmt.Println(global.DB.Model(&User{}).Statement.Table)         // 输出: users
func GetExchangeRates(c *gin.Context) { //这里采用结构体切片来操作
	var exchangeRates []models.ExchangeRate
	if err := global.DB.Table("exchange_rates").Find(&exchangeRates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	c.JSON(200, exchangeRates) // 返回结构体数据
}

func GetUserName(c *gin.Context) {  //展示当前界面的用户名称
	name, flag := c.Get("username")
	if flag {
		c.JSON(200, gin.H{"username": name})
	} else {
		c.JSON(200, gin.H{"username": "unknown"})
	}
}

func Get_advertisement(c *gin.Context) { //展示当前界面的广告
    url, flag := c.Get("my_blog")
	if flag {
		c.JSON(200, gin.H{"author_url": url})
	} else {
		c.JSON(200, gin.H{"authorurl": "unknown"})
	} 
}
