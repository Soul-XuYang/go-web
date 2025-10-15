// controllers/fx_top10.go
package controllers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"project/global"
	"project/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var top10Symbols = []string{"USD", "EUR", "JPY", "GBP", "AUD", "CAD", "CHF", "HKD", "SGD", "KRW"}

type frankResp struct { //接受数据
	Base  string             `json:"base"` // 有些版本字段为 "base"；若实际为 "from"，见下方容错
	From  string             `json:"from"` // 兼容 Frankfurter 新老字段
	Date  string             `json:"date"` // "YYYY-MM-DD"
	Rates map[string]float64 `json:"rates"`
}

// 四舍五入的有效n位小数
const vaild_number = 6

func roundN(x float64, n int) float64 { // 浮点数四舍五入到指定小数位数的工具函数
	if math.IsNaN(x) || math.IsInf(x, 0) { //math.IsInf(x, 0) 是Go语言中用来检查浮点数是否为无穷大的函数
		return 0
	}
	p := math.Pow10(n)         //
	return math.Round(x*p) / p //先放大再舍去小数部分最后缩小
}

// RefreshRmbTop10 godoc
// @Summary     手动刷新人民币 Top10 汇率
// @Tags        Exchange
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  map[string]string
// @Router      /rmb-top10/refresh [post]
func RefreshRmbTop10(c *gin.Context) {
	// Frankfurter 兼容两种写法：?base= / ?from=
	// 建议优先使用 from/to
	url := "https://api.frankfurter.dev/v1/latest?from=CNY&to=" + strings.Join(top10Symbols, ",") //查询
	//https://api.frankfurter.dev/v1/latest?from=CNY&to=USD,EUR,JPY,GBP,AUD,CAD,CHF,HKD,SGD,KRW
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "ExchangeApp/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "fetch rates failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("upstream %d", resp.StatusCode)})
		return
	}

	var fr frankResp
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "decode json failed: " + err.Error()})
		return
	}

	// 解析日期
	asOf, _ := time.Parse("2006-01-02", fr.Date)

	tx := global.DB.Begin()

	// ✅ 用 GORM 删除，避免表名大小写/复数化差异
	if err := tx.Where("1=1").Delete(&models.RmbTop10S{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 写入
	for sym, r := range fr.Rates {
		rate := r
		// 正向保留 6 位，反向同样 6 位（需要 4 位可把 roundN 的 n 改成 4）
		rate = roundN(rate, vaild_number)

		inv := 0.0
		if rate > 0 {
			inv = roundN(1.0/rate, vaild_number)
		}

		row := models.RmbTop10S{
			Symbol: strings.ToUpper(sym),
			Rate:   rate,
			Invert: inv,
			AsOf:   asOf,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	//提交数据
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":    true,
		"base":  "CNY",
		"as_of": fr.Date,
		"count": len(fr.Rates),
	})
}

type rmbTop10View struct {
	Symbol string `json:"symbol"`
	Rate   string `json:"rate"`   // 字符串化，避免前端精度/地区格式问题
	Invert string `json:"invert"` // 同上
	AsOf   string `json:"as_of"`  //地区国家
}

// GetRmbTop10 godoc
// @Summary     读取当前人民币对Top10地区的汇率快照
// @Tags        Exchange
// @Security    Bearer
// @Produce     json
// @Success     200   {array}   map[string]interface{}
// @Router      /api/rmb-top10 [get]
// 读取当前快照（按符号排序）—— 返回字符串化数值，后台的数据类型转换
func GetRmbTop10(c *gin.Context) {
	var list []models.RmbTop10S
	if err := global.DB.Order("symbol ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]rmbTop10View, 0, len(list))
	for _, it := range list {
		// 保底：若历史数据 invert 为 0，但 rate>0，则动态补算一次
		inv := it.Invert
		if inv == 0 && it.Rate > 0 {
			inv = roundN(1.0/it.Rate, 6)
		}
		out = append(out, rmbTop10View{
			Symbol: it.Symbol,
			Rate:   fmt.Sprintf("%.6f", it.Rate),
			Invert: fmt.Sprintf("%.6f", inv),
			AsOf:   it.AsOf.Format("2006-01-02"),
		})
	}
	c.JSON(http.StatusOK, out)
}
