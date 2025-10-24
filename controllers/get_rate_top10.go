// controllers/fx_top10.go
package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"project/config"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RmbTop10S
var top10Symbols = []string{"USD", "EUR", "JPY", "GBP", "AUD", "CAD", "CHF", "HKD", "SGD", "KRW"}

type frankResp struct { //响应数据的结构
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

const (
	lock_Ratekey = "lock:rmb_top10:cny"
)

// API view model
type rmbTop10View struct {
	Symbol string `json:"symbol"`
	Rate   string `json:"rate"`
	Invert string `json:"invert"`
	AsOf   string `json:"as_of"`
}

type rmbTop10Cache struct {
	Base string         `json:"base"`  // 这里的base指的就是CNY
	AsOf string         `json:"as_of"` // YYYY-MM-DD
	List []rmbTop10View `json:"list"`
}

// RefreshRmbTop10
// @Summary 手动刷新人民币 Top10 汇率（写入 Redis，TTL=24h）
// @Tags Exchange
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]string
// @Router /rmb-top10/refresh [post]
func RefreshRmbTop10(c *gin.Context) {
	ctx := c.Request.Context()
	ok, err := acquireLock(ctx, lock_Ratekey, config.LockTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lock error: " + err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "another refresh in progress"})
		return
	}
	defer releaseLock(ctx, lock_Ratekey) //释放锁

	cache, err := fetchAndBuild_top10rates(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := setCache(ctx, config.Cache_RateKey, cache); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache set failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":    true,
		"base":  cache.Base,
		"as_of": cache.AsOf,
		"count": len(cache.List),
	})
}

// GetRmbTop10
// @Summary 读取当前人民币对Top10地区的汇率快照（来自 Redis）
// @Tags Exchange
// @Security Bearer
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /api/rmb-top10 [get]
func GetRmbTop10(c *gin.Context) {
	ctx := c.Request.Context() //获得当前请求的context以构建何时都可以退出的情况

	if cache, err := getCache(ctx, config.Cache_RateKey); err == nil { // 获取缓存数据
		c.JSON(http.StatusOK, cache.List)
		return
	}

	// 缓存缺失：尝试成为唯一回源者
	if ok, _ := acquireLock(ctx, lock_Ratekey, config.LockTTL); ok { //获取分布锁的函数，如果key不存在返回true这里之后可以并发锁住，在构建时锁住
		defer releaseLock(ctx, lock_Ratekey)        // 最后释放锁
		cache, err := fetchAndBuild_top10rates(ctx) //拿去以及创建
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		// 这里创建缓存-将数据存入redis表中
		if err := setCache(ctx, config.Cache_RateKey, cache); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cache set failed: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, cache.List)
		return
	} //这里释放

	// 获取锁失败时
	// 其它实例正在刷新：短暂等待缓存被填充-轮询查询模式
	deadline := time.Now().Add(config.WaitWarmup) //当前时间加上配置时间
	for time.Now().Before(deadline) {
		jitter := time.Duration(time.Now().UnixNano()%40) * time.Millisecond //使用当前纳秒时间对40取模，确保随机性
		time.Sleep(config.PollInterval + jitter)                             //等待时间+随机时间
		if cache, err := getCache(ctx, config.Cache_RateKey); err == nil {   //如果读取缓存可以的话就走
			c.JSON(http.StatusOK, cache.List)
			return
		}
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache warming, please retry"})
}

// 获取汇率函数
func fetchAndBuild_top10rates(ctx context.Context) (*rmbTop10Cache, error) {
	url := "https://api.frankfurter.dev/v1/latest?from=CNY&to=" + strings.Join(top10Symbols, ",")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //后端设定请求
	req.Header.Set("User-Agent", "ExchangeApp/1.0")                     //设置头以便更好获取数据

	resp, err := http.DefaultClient.Do(req) //执行该请求
	if err != nil {
		return nil, fmt.Errorf("fetch rates failed: %w", err)
	}
	defer resp.Body.Close() //响应体关闭

	if resp.StatusCode != http.StatusOK { //状态码报错
		return nil, fmt.Errorf("upstream %d", resp.StatusCode)
	}

	var fr frankResp
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil { //解码其数据
		return nil, fmt.Errorf("decode json failed: %w", err)
	}

	base := fr.From // 从查询
	if base == "" { //从兑换结果找
		base = fr.Base
	}

	asOfTime, _ := time.Parse("2006-01-02", fr.Date) //解码时间

	list := make([]rmbTop10View, 0, len(fr.Rates)) //起始长度为0，容量为len
	for sym, r := range fr.Rates {
		rate := roundN(r, vaild_number)       //化为有效数据
		inv := roundN(1.0/rate, vaild_number) //反数据
		if inv <= 0 {
			return nil, fmt.Errorf("invalid rate and inv data")
		}
		list = append(list, rmbTop10View{ //加入数据
			Symbol: strings.ToUpper(sym),
			Rate:   fmt.Sprintf("%.6f", rate),
			Invert: fmt.Sprintf("%.6f", inv),
			AsOf:   asOfTime.Format("2006-01-02"),
		})
	}
	return &rmbTop10Cache{Base: base, AsOf: asOfTime.Format("2006-01-02"), List: list}, nil
}
