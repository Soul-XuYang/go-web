package controllers

import (
	"fmt"
	"net/http"
	"project/global"
	"project/log"
	"project/models"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 这里超级管理员个数比较少，所以这里不怎么做并发的优化

// TotalData 仪表盘汇总数据
// @Description 仪表盘顶部各项总数
type totalData struct {
	TotalUsers           int64 `json:"totalUsers" example:"1234"`          // 用户总数
	TotalArticles        int64 `json:"totalArticles" example:"567"`        // 文章总数
	TotalCollections     int64 `json:"totalCollections" example:"89"`      // 收藏夹总数
	TotalCollectionItems int64 `json:"totalCollectionItems" example:"345"` // 收藏夹条目总数
	TotalReposts         int64 `json:"totalReposts" example:"12"`          // 转发总数
	TotalLikes           int64 `json:"totalLikes" example:"3456"`          // 点赞总数
	TotalFiles           int64 `json:"totalFiles" example:"78"`            // 文件总数
	TotalGame2048Score   int64 `json:"totalGame2048Score" example:"9999"`  // 2048 总分
	TotalGameGuessScore  int64 `json:"totalGameGuessScore" example:"8888"` // 猜数字总分
	TotalGameMapTime     int64 `json:"totalGameMapTime" example:"12345"`   // 地图游戏总时长（单位自定）
}

// 权限管理-只有管理员可以访问
// GetDashboardTotalData
// @Summary 仪表盘-汇总数据
// @Description 返回各模型的总数统计（仅管理员可访问）
// @Tags dashboard
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} TotalData
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/dashboard/total [get]
func GetDashboardTotalData(c *gin.Context) {
	userID := c.GetUint("user_id")
	Role := c.GetString("role")
	if userID == 0 || Role == "user" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}

	// 定义模型映射
	modelMap := map[string]interface{}{
		"users":                   &models.Users{},
		"articles":                &models.Article{},
		"collections":             &models.Collection{},
		"collectionItems":         &models.CollectionItem{},
		"reposts":                 &models.UserArticleRepost{},
		"likes":                   &models.UserLikeArticle{},
		"Files":                   &models.Files{},
		"Game_2048_Score":         &models.Game_2048_Score{},
		"Game_Guess_Number_Score": &models.Game_Guess_Score{},
		"Game_Map_Time":           &models.Game_Map_Time{},
	}
	type countResult struct { //构建一个结构体计算即可
		Name  string
		Count int64
	}
	// 创建结果通道-并行查询
	resultChan := make(chan countResult, len(modelMap)) //创建对应的映射数据
	errorChan := make(chan error, len(modelMap))
	var wg sync.WaitGroup //同步计算
	var mu sync.Mutex
	// 使用 goroutine 并行查询
	for name, model := range modelMap {
		wg.Add(1)
		//每一个循环查询一个数据
		go func(name string, model interface{}) {
			defer wg.Done()
			mu.Lock()
			var count int64
			err := global.DB.Model(model).Count(&count).Error //查询
			if err != nil {
				errorChan <- fmt.Errorf("failed to count %s: %v", name, err)
				return
			}
			resultChan <- countResult{Name: name, Count: count} //创建对应的哈希表并传入
			mu.Unlock()
		}(name, model) //传入对应的参数
	}
	wg.Wait() //等待所有结束即可
	close(resultChan)
	close(errorChan)

	// 存在错误，返回错误信息
	if len(errorChan) > 0 {
		for e := range errorChan {
			log.L().Error("GetDashboardTotalData error:", zap.Error(e))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询数据失败"})
		return
	}
	// 收集结果
	totalData := &totalData{}
	for result := range resultChan { //注意这里没有顺序的差异索引无需索引
		switch result.Name {
		case "users":
			totalData.TotalUsers = result.Count
		case "articles":
			totalData.TotalArticles = result.Count
		case "collections":
			totalData.TotalCollections = result.Count
		case "collectionItems":
			totalData.TotalCollectionItems = result.Count
		case "reposts":
			totalData.TotalReposts = result.Count
		case "likes":
			totalData.TotalLikes = result.Count
		case "Files":
			totalData.TotalFiles = result.Count
		case "Game_2048_Score":
			totalData.TotalGame2048Score = result.Count
		case "Game_Guess_Number_Score":
			totalData.TotalGameGuessScore = result.Count
		case "Game_Map_Time":
			totalData.TotalGameMapTime = result.Count
		}
	}
	c.JSON(http.StatusOK, totalData)
}

// 时间统计
type TimeRange string

const (
	Last7Days   TimeRange = "last7days" //
	Last6Months TimeRange = "last6months"
	Last3Months TimeRange = "last3months"
	LastDay     TimeRange = "lastday"
	Custom      TimeRange = "customStart" // 最近，可以自定义时间范围
)

// 添加customStart来进行用户定制的时间
func getNewTimeCount(db *gorm.DB, table string, timeRange TimeRange, customStart time.Time) (int64, error) {
	var startTime time.Time
	now := time.Now() //获取当前的时间

	// 根据时间范围计算开始时间
	switch timeRange {
	case Last7Days:
		startTime = now.AddDate(0, 0, -7)
	case Last3Months:
		startTime = now.AddDate(0, -3, 0)
	case Last6Months:
		startTime = now.AddDate(0, -6, 0)
	case LastDay:
		startTime = now.AddDate(0, 0, -1)
	case Custom:
		startTime = customStart
	default:
		return 0, fmt.Errorf("invalid time range")
	}
	var count int64
	err := db.Table(table).Where("created_at >= ? AND created_at <= ?", startTime, now).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to query articles count: %v", err)
	}
	return count, nil
}

// DashboardAdd 仪表盘新增数据
// @Description 依次返回以下三个时间窗口内的新增数量：Last7Days、Last3Months、LastDay
type DashboardAdd struct {
	// 用户新增数量序列（顺序：7天、3个月、1天）
	// example: [12, 120, 1]
	User_add []int64 `json:"user_add"`
	// 文件新增数量序列（顺序：7天、3个月、1天）
	// example: [34, 560, 3]
	Files_add []int64 `json:"files_add"`
	// 文章新增数量序列（顺序：7天、3个月、1天）
	// example: [7, 98, 0]
	Articles_add []int64 `json:"articles_add"`
}

// GetUser_Info -这里有现成的函数直接返回管理员的信息
// GetDashboardAdd
// @Summary 仪表盘-新增统计
// @Description 返回用户/文件/文章在 Last7Days、Last3Months、LastDay 三个窗口内的新增数量（按此顺序）
// @Tags dashboard
// @Produce json
// @Security Bearer
// @Success 200 {object} DashboardAdd
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/dashboard/add [get]
func GetDashboardAdd(c *gin.Context) {
	userID := c.GetUint("user_id")
	Role := c.GetString("role")
	if userID == 0 || Role == "user" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}
	times := []TimeRange{Last7Days, Last3Months, LastDay} //初始化数组
	user_add := []int64{}
	files_add := []int64{}
	articles_add := []int64{}
	// 遍历时间变量
	for _, t := range times {
		count, err := getNewTimeCount(global.DB, "users", t, time.Time{})
		if err != nil {
			log.L().Error("Failed to get user count", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户数据失败"})
			return
		}
		user_add = append(user_add, count)

		count, err = getNewTimeCount(global.DB, "files", t, time.Time{})
		if err != nil {
			log.L().Error("Failed to get files count", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文件数据失败"})
			return
		}
		files_add = append(files_add, count)

		count, err = getNewTimeCount(global.DB, "articles", t, time.Time{})
		if err != nil {
			log.L().Error("Failed to get articles count", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文章数据失败"})
			return
		}
		articles_add = append(articles_add, count)
	}
	c.JSON(http.StatusOK, DashboardAdd{
		User_add:     user_add,
		Files_add:    files_add,
		Articles_add: articles_add,
	})
}

// CurveInput 请求体
// @Description 折线数据请求体
type curveInput struct {
	// 时间范围，格式如 "7days" / "6months" / "1year"
	// swagger:enum
	// example: 7days
	TimeRange string `json:"time_range" binding:"required" example:"7days"`
	// 统计的数据表
	// swagger:enum
	// enum: users,files,articles
	// example: users
	Table string `json:"table" binding:"required,oneof=users files articles" example:"users"`
}

type dailyData struct {
	// 时间维度: day / month / year
	// enum: day,month,year
	TimeDimension string `json:"time_dimension" example:"day"`
	// 数量序列（按时间从旧到新）
	Data []int64 `json:"data"`
	// 序列长度
	DataLength int `json:"data_length" example:"7"`
}

// GetDashboardCurveData
// @Summary 仪表盘折线数据
// @Description 按 day/month/year 维度返回指定表在各时间段的新增数量
// @Tags dashboard
// @Accept json
// @Produce json
// @Param request body CurveInput true "请求体"
// @Success 200 {object} DailyData
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/dashboard/curve [post]
func GetDashboardCurveData(c *gin.Context) {
	userID := c.GetUint("user_id")
	Role := c.GetString("role")
	if userID == 0 || Role == "user" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}
	var input curveInput //获得输入的数据
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	var length int
	var err error
	var time_dimension string
	if strings.Contains(string(input.TimeRange), "days") {
		// 获取数字部分
		numStr := strings.TrimSuffix(string(input.TimeRange), "days") // 删除字符串末尾的后缀-这里写length, _ = strconv.Atoi(string(input.TimeRange[0]))不对，因为数字可能不是0-9的一位数字
		length, err = strconv.Atoi(numStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid number format"})
			return
		}
		time_dimension = "day"
	} else if strings.Contains(string(input.TimeRange), "months") {
		// 获取数字部分
		numStr := strings.TrimSuffix(string(input.TimeRange), "months")
		length, err = strconv.Atoi(numStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid number format"})
			return
		}
		time_dimension = "month"
	} else if strings.Contains(string(input.TimeRange), "years") {
		// 获取数字部分
		numStr := strings.TrimSuffix(string(input.TimeRange), "years")
		length, err = strconv.Atoi(numStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid number format"})
			return
		}
		time_dimension = "year"
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range data"})
		return
	}

	now := time.Now()
	var date time.Time
	var startTime, endTime time.Time
	dailyChan := make(chan int64, length) // length对应长度
	errorChan := make(chan error, length)
	results := make([]int64, length)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := length - 1; i >= 0; i-- { // 倒序，这里长度是间隔
		switch time_dimension {
		case "day":
			date = now.AddDate(0, 0, -i)
			startTime = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
			endTime = time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 0, date.Location())

		case "month":
			date = now.AddDate(0, -i, 0)
			// 获取月份的第一天
			startTime = time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
			// 获取月份的最后一天
			endTime = time.Date(date.Year(), date.Month()+1, 0, 23, 59, 59, 0, date.Location())

		case "year":
			date = now.AddDate(-i, 0, 0)
			// 获取年份的第一天
			startTime = time.Date(date.Year(), 1, 1, 0, 0, 0, 0, date.Location())
			// 获取年份的最后一天
			endTime = time.Date(date.Year(), 12, 31, 23, 59, 59, 0, date.Location())
		}
		wg.Add(1)
		// 查询当前时间段的新增用户数
		go func(idx int, start, end time.Time) {
			defer wg.Done()
			var Count int64
			mu.Lock() //这里加锁保证对应的索引写对
			err := global.DB.Table(input.Table).
				Where("created_at >= ? AND created_at <= ?", start, end).
				Count(&Count).Error
			if err != nil {
				log.L().Error("Failed to get count", zap.Error(err))
				errorChan <- err
				return
			}
			results[idx] = Count
			mu.Unlock()
		}(i, startTime, endTime)
	}
	wg.Wait()
	close(dailyChan)
	close(errorChan)
	if len(errorChan) > 0 {
		for e := range errorChan {
			log.L().Error("GetDashboardTotalData error:", zap.Error(e))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询数据失败"})
		return
	}
	c.JSON(http.StatusOK, &dailyData{
		TimeDimension: time_dimension, //day month year
		Data:          results,
		DataLength:    length,
	})
}
