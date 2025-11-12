package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"project/config"
	"project/global"
	"project/log"
	"project/models"
	"project/utils"
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
	TotalUsers           int64  `json:"totalUsers" example:"1234"`          // 用户总数
	TotalArticles        int64  `json:"totalArticles" example:"567"`        // 文章总数
	TotalCollections     int64  `json:"totalCollections" example:"89"`      // 收藏夹总数
	TotalCollectionItems int64  `json:"totalCollectionItems" example:"345"` // 收藏夹条目总数
	TotalReposts         int64  `json:"totalReposts" example:"12"`          // 转发总数
	TotalLikes           int64  `json:"totalLikes" example:"3456"`          // 点赞总数
	TotalFiles           int64  `json:"totalFiles" example:"78"`            // 文件总数
	TotalGame2048Score   int64  `json:"totalGame2048Score" example:"9999"`  // 2048 总分
	TotalGameGuessScore  int64  `json:"totalGameGuessScore" example:"8888"` // 猜数字总分
	TotalGameMapTime     int64  `json:"totalGameMapTime" example:"12345"`   // 地图游戏总时长（单位自定）
	Version              string `json:"version"`
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
	totalData.Version = config.Version
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
	errorChan := make(chan error, length)
	results := make([]int64, length)
	var wg sync.WaitGroup
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
		orderIndex := length - 1 - i
		wg.Add(1)
		// 查询当前时间段的新增用户数-并且有对应的索引
		go func(idx int, start, end time.Time) {
			defer wg.Done()
			var Count int64
			// 数据库查询不需要加锁，因为每个goroutine写入的是不同的索引
			err := global.DB.Table(input.Table).
				Where("created_at >= ? AND created_at <= ?", start, end).
				Count(&Count).Error
			if err != nil {
				log.L().Error("Failed to get count", zap.Error(err))
				errorChan <- err
				return
			}
			results[idx] = Count
		}(orderIndex, startTime, endTime)
	}
	wg.Wait()
	close(errorChan)
	// 检查是否有错误
	hasError := false
	for e := range errorChan {
		hasError = true
		log.L().Error("GetDashboardCurveData error:", zap.Error(e))
	}
	if hasError {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询数据失败"})
		return
	}
	c.JSON(http.StatusOK, &dailyData{
		TimeDimension: time_dimension, //day month year
		Data:          results,
		DataLength:    length,
	})
}

// 统计后台时间
type TimeInfo struct {
	CurrentTime  string `json:"currentTime"  example:"2025-11-11 10:25"` // 当前时间（到分钟）
	SystemUptime string `json:"systemUptime" example:"0天 01小时:23分"`      // 系统运行时长（易读）
	StartTime    string `json:"startTime"    example:"2025-11-11 10:25"`
}

// 计算系统运行时间
func getSystemUptime() time.Duration {
	return time.Since(config.StartTime)
}

// 格式化持续时间
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%d天 %02d小时:%02d分", days, hours, minutes)
}

const timeLayoutMinute = "2006-01-02 15:04"
const timeLayoutSecond = "2006-01-02 15:04:05"

// GetDashboardTimeInfo
// @Summary 仪表盘-时间心跳（WebSocket）
// @Description 这里用SSE协议，每 60 秒推送一次当前时间与系统运行时长；连接成功后立即推送一条。服务器会定期发送 ping 维持长连。
// @Tags dashboard
// @Security Bearer
// @Produce json
// @Success 101 {string} string "Switching Protocols"
// @Failure 401 {object} map[string]string
// @Router /dashboard/time/sse [get]
func GetDashboardTimeInfo(c *gin.Context) {
	// SSE链接
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream") //这里告诉前端浏览器这是SSE流为SSE
	h.Set("Cache-Control", "no-cache")         //无需缓存
	h.Set("Connection", "keep-alive")          //保持长连接
	h.Set("X-Accel-Buffering", "no")           // Nginx 关闭缓冲

	flusher, ok := c.Writer.(http.Flusher) //接口断言-flusher为接口的使用
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stream not supported in InternalServer"})
		return
	}

	ctx := c.Request.Context() //设置文本上下文
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	sendmsg := func() { //设定当前的函数
		now := time.Now()
		payload := TimeInfo{
			CurrentTime:  now.Format(timeLayoutMinute),
			SystemUptime: formatDuration(getSystemUptime()),
			StartTime:    config.StartTime.Format(timeLayoutSecond),
		}
		body, _ := json.Marshal(payload) //结构体数据转为json数据
		// SSE 事件：名称为 tick
		fmt.Fprintf(c.Writer, "event: tick\n")      //告诉前端，前端可用 es.addEventListener('tick', handler) 监听
		fmt.Fprintf(c.Writer, "data: %s\n\n", body) //告诉前端，前端可用 es.addEventListener('data', handler) 监听
		// 必须以
		flusher.Flush() // 必须以空行结束（\n\n），表示这一条 SSE 事件结束；浏览器才能按条解析-将缓存区的数据发送出去
	}

	sendmsg()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendmsg()
		}
	}
}

// == 个人用户的管理函数 == 专属于superadmin用户的管理函数 ==
type UserList struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}
type UserListResponse struct {
	Items []UserList `json:"items"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

// @Summary 获取用户列表
// @Description 获取用户列表，支持分页和排序
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param order query string false "排序方式" Enums(created_asc,created_desc) default(created_desc)
// @Success 200 {object} UserListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /api/dashboard/user [get]
func GetUserList(c *gin.Context) {
	userID := c.GetUint("user_id")
	Role := c.GetString("role")
	if userID == 0 || Role == "user" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}
	//页数管理操作
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	order := strings.TrimSpace(c.Query("order"))
	var users []models.Users
	var total int64
	// 查询总数
	if err := global.DB.Model(&models.Users{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户总数失败"})
		return
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	db := global.DB.Model(&models.Users{})
	switch order {
	case "created_asc":
		db = db.Order("created_at ASC")
	case "created_desc":
		db = db.Order("created_at DESC")
	default:
		db = db.Order("created_at DESC") //默认倒序
	}
	db = db.Select("id, username, role,status, created_at") //获取用户列表所需的信息
	if err := db.Offset((page - 1) * size).Limit(size).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	items := make([]UserList, len(users))
	for i, u := range users {
		items[i] = UserList{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			Status:    u.Status,
			CreatedAt: u.CreatedAt.Unix(),
		}
	}
	c.JSON(http.StatusOK, &UserListResponse{ //方便前端统计数据
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

// deleteUserDTO 删除操作的响应结构
type deleteUserDTO struct {
	Deleted bool `json:"deleted" example:"true"` // 是否删除成功
}

// DeleteUserFromDashboard
// @Summary 删除用户
// @Description 从仪表盘删除指定ID的用户（仅管理员可访问）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Security Bearer
// @Success 200 {object} deleteUserDTO
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/dashboard/user/{id} [delete]
func DeleteUserFromDashboard(c *gin.Context) {
	userID := c.GetUint("user_id")
	Role := c.GetString("role")
	if userID == 0 || Role == "user" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}
	DeleteduserID := c.Param("id")
	if ID, err := strconv.Atoi(DeleteduserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	} else {
		if err := global.DB.Delete(&models.Users{}, ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户失败"})
			return
		}
	}
	c.JSON(http.StatusOK, &deleteUserDTO{Deleted: true})
}

// userUpdateDTO 定义用户更新请求的数据传输对象
// 使用binding标签进行数据验证：
// - min/max: 字符串长度限制
// - oneof: 限定字段值必须是给定选项之一
type userUpdateDTO struct {
	Username string `json:"username" binding:"min=1,max=20"` // 用户名，长度1-20字符
	Role     string `json:"role" binding:"oneof=admin user"` // 用户角色，只能是admin或user
	Status   string `json:"status"`                          // 用户状态，非必填字段
}

// UpdateUser
// @Summary 更新用户信息
// @Description 管理员更新指定用户的信息，包括用户名、角色和状态
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Param data body userUpdateDTO true "用户更新信息"
// @Success 200 {object} map[string]string "更新成功"
// @Failure 400 {object} map[string]string "请求参数错误"
// @Failure 401 {object} map[string]string "未授权访问"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /api/dashboard/user/{id} [put]
// @Security ApiKeyAuth
func UpdateUser(c *gin.Context) { //这里请求是put并且接收参数
	userID := c.GetUint("user_id")
	Role := c.GetString("role")
	if userID == 0 || Role == "user" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}
	var input userUpdateDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if input.Role != "admin" && input.Role != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户的权限设置错误，只能为admin或user"})
		return
	}
	if err := global.DB.Model(&models.Users{}).Where("id = ?", c.Param("id")).Updates(input).Error; err != nil { //操作的数据使用结构体来操作
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户信息失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "用户信息更新成功"})
}

// addUserReqDTO 添加用户请求参数
type addUserReqDTO struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"oneof=admin user"`
	Status   string `json:"status"`
}

// addUserResDTO 添加用户响应结果
type addUserResDTO struct {
	Username string `json:"username" binding:"required"`
	Role     string `json:"role" binding:"oneof=admin user"`
	Status   string `json:"status"`
	ID       uint   `json:"id"`
}

// AddUser
// @Summary 添加新用户
// @Description 管理员添加新用户，需要提供用户名、密码、角色等信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param data body addUserReqDTO true "用户信息"
// @Success 200 {object} addUserResDTO "添加成功"
// @Failure 400 {object} map[string]string "请求参数错误"
// @Failure 401 {object} map[string]string "未授权访问"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /api/dashboard/user/{id} [post]
// @Security ApiKeyAuth
func AddUser(c *gin.Context) {
	userID := c.GetUint("user_id")
	Role := c.GetString("role")
	if userID == 0 || Role == "user" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}
	var input addUserReqDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if input.Role != "admin" && input.Role != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户的权限设置错误，只能为admin或user"})
		return
	}
	var count int64
	if err := global.DB.Model(&models.Users{}).
		Where("username = ?", input.Username).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户已存在"})
		return
	}
	hashPassword, err := utils.HashPassword(input.Password) // 对其加密
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
		return
	}
	user := &models.Users{
		Username: input.Username,
		Password: hashPassword,
		Role:     input.Role,
		Status:   input.Status,
	}
	if err := global.DB.Create(&user).Error; err != nil { //GORM的Create()方法可以接受多种类型的参数-指针、结构体、切片、Map类型
		// 指针可以获取创建后的ID值 可以获取其他数据库自动生成的字段值 性能更好（避免值拷贝）
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, &addUserResDTO{
		Username: user.Username,
		Role:     user.Role,
		Status:   user.Status,
		ID:       user.ID,
	})
}
