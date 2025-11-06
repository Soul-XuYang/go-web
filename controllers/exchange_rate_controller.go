package controllers

import (
	"net/http"
	"project/global"
	"project/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 汇率数据
// CreateExchangeRate godoc
// @Summary     获取当前汇率数据
// @Tags        Me
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  map[string]string  "示例：{\"username\":\"alice\"}"
// @Router      /exchangeRates [post]
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

// 可以用这种方式查看表-但是这种返回的是全体页数需要注意
// fmt.Println(global.DB.Model(&ExchangeRate{}).Statement.Table) // 输出: exchange_rates
// fmt.Println(global.DB.Model(&User{}).Statement.Table)         // 输出: users

func GetExchangeRates(c *gin.Context) { //这里采用结构体切片来操作
	var exchangeRates []models.ExchangeRate
	if err := global.DB.Table("exchange_rates").Find(&exchangeRates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	c.JSON(200, exchangeRates) // 返回结构体数据
}

// GetUserName godoc
// @Summary     获取当前用户名
// @Tags        Me
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  map[string]string  "示例：{\"username\":\"alice\"}"
// @Router      /me [get]
func GetUserName(c *gin.Context) { //展示当前界面的用户名称
	name, flag := c.Get("username")
	if flag {
		c.JSON(200, gin.H{"username": name})
	} else {
		c.JSON(200, gin.H{"username": "unknown"})
	}
}

// Get_advertisement godoc
// @Summary     获取作者博客广告
// @Tags        Me
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  map[string]string   "示例：{\"author_url\":\"https://...\"}"
// @Router      /ad [get]
func Get_advertisement(c *gin.Context) { //展示当前界面的广告
	url, flag := c.Get("my_blog")
	if flag {
		c.JSON(200, gin.H{"author_url": url})
	} else {
		c.JSON(200, gin.H{"authorurl": "unknown"})
	}
}

// DeleteExchangeRate godoc
// @Summary 删除汇率记录
// @Description 根据ID删除指定的汇率记录
// @Tags ExchangeRate
// @Accept json
// @Produce json
// @Param id path int true "汇率记录ID"
// @Success 200 {object} map[string]interface{} "成功删除"
// @Failure 400 {object} map[string]interface{} "无效的ID"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 404 {object} map[string]interface{} "汇率记录不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Security ApiKeyAuth
// @Router       /api/exchangeRates/{id} [delete]
func DeleteExchangeRate(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	//获取
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid exchangeRate id"})
		return
	}

	//删除前先判断是否是存在这条记录
	var exchangeRate models.ExchangeRate
	if err := global.DB.Where("id = ?", id).First(&exchangeRate).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "this exchangeRate not found"})
		return
	}
	err = global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&exchangeRate).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete article and related data"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "deleted successfully"})
}

type updateRate struct {
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	Rate         float64 `json:"rate"`
}
type updateRateResponse struct {
	ID           uint64    `json:"id"`
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"`
	Date         time.Time `json:"date"`
}

// UpdateRate godoc
// @Summary     更新汇率记录
// @Description 根据ID更新指定的汇率记录信息
// @Tags        ExchangeRate
// @Accept      json
// @Produce     json
// @Param       id   path      int                true  "汇率记录ID"
// @Param       request body     updateRate          true  "更新汇率请求体"
// @Success     200   {object}  updateRateResponse  "成功更新"
// @Failure     400   {object}  map[string]interface{}  "无效的请求参数"
// @Failure     401   {object}  map[string]interface{}  "未授权"
// @Failure     500   {object}  map[string]interface{}  "服务器内部错误"
// @Security    ApiKeyAuth
// @Router      /api/exchangeRates/{id} [put]
func UpdataRate(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid exchangeRate id"})
		return
	}

	var req updateRate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	} //接受请求的数据

	//这里update建议不要用if一一判断太麻烦了，注意update可以直接用哈希表来传递的-之后的文章更新也是一样的操作
	map_updateExchangeRate := make(map[string]interface{}) //创建一个map
	if req.FromCurrency != "" {
		map_updateExchangeRate["from_currency"] = req.FromCurrency
	}
	if req.ToCurrency != "" {
		map_updateExchangeRate["to_currency"] = req.ToCurrency
	}
	if req.Rate != 0 {
		map_updateExchangeRate["rate"] = req.Rate
	}
	if err := global.DB.Model(&models.ExchangeRate{}).Where("id = ?", id).Updates(map_updateExchangeRate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	res := &updateRateResponse{
		ID:           id,
		FromCurrency: req.FromCurrency,
		ToCurrency:   req.ToCurrency,
		Rate:         req.Rate,
		Date:         time.Now(),
	}
	c.JSON(http.StatusOK, res)
}
