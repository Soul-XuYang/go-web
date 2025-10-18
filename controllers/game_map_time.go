package controllers

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"project/config"
	"project/global"
	"project/log"
	"project/models"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var game_rounds = [3]int{8, 12, 16}                   // 不同难度对应的大小
var dir = [4][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} // 四个方向

const map_users_number = 10 // 每个用户最多保存5条记录

type P struct { // 坐标点-保持json映射关系
	X int `json:"x"`
	Y int `json:"y"`
}

// 地图游戏玩家状态-resp数据
type MapGamePlayer struct {
	Round            int       `json:"round"`            // 当前轮次：1, 2, 3
	Difficulty       int       `json:"difficulty"`       // 当前轮次难度等级：0, 1, 2
	MapData          [][]byte  `json:"mapData"`          // 地图数据
	StartPoint       P         `json:"startPoint"`       // 起点
	EndPoint         P         `json:"endPoint"`         // 终点
	RoundStartTime   time.Time `json:"roundStartTime"`   // 当前轮开始时间
	GameStartTime    time.Time `json:"gameStartTime"`    // 整局游戏开始时间
	TotalTime        float64   `json:"totalTime"`        // 累计总时间（秒）
	IsRoundCompleted bool      `json:"isRoundCompleted"` // 当前轮是否完成
}

// 创建全局变量-mapGameState表-玩家表
type mapGameState struct {
	Players map[uint]*MapGamePlayer // id映射玩家表
	mu      sync.Mutex              // 互斥锁
}

var mapGame = &mapGameState{ // 创建全局变量-mapGameState表
	Players: make(map[uint]*MapGamePlayer),
}

// ---------------------------------------

// 初始化地图游戏玩家-重置
func init_MapGamePlayer() *MapGamePlayer { // 	初始化玩家状态-返回玩家指针
	return &MapGamePlayer{
		Round:            1,
		Difficulty:       0, // 第1轮从简单开始
		GameStartTime:    time.Now(),
		RoundStartTime:   time.Now(),
		TotalTime:        0,
		IsRoundCompleted: false,
	}
}

// 根据轮次获取对应的难度
func getDifficultyForRound(round int) int {
	if round <= 0 || round > 3 {
		log.L().Error("DifficultyForRound failed", zap.Int("round", round))
		return 0
	}
	return round - 1 // 第1轮=难度0，第2轮=难度1，第3轮=难度2
}

// 获取难度名称
func getDifficultyName(difficulty int) string {
	switch difficulty {
	case 0:
		return "简单"
	case 1:
		return "中等"
	case 2:
		return "困难"
	default:
		return "简单"
	}
}

/********* DTO数据-响应数据 *********/
// 易错，因为json会把byte[]映射成string，所以需要用[]string,因此需要改成strinf
type startMapGameResp struct {
	Message         string   `json:"message"`
	Round           int      `json:"round"`
	Difficulty      int      `json:"difficulty"`
	Size            int      `json:"size"`
	MapData         []string `json:"mapData"` // ← 改这里
	StartPoint      P        `json:"startPoint"`
	EndPoint        P        `json:"endPoint"`
	CurrentDistance int      `json:"currentDistance"`
	TotalTime       float64  `json:"totalTime"`
}

type completeMapGameResp struct { // 返回完成游戏的数据
	Message      string  `json:"message"`
	Round        int     `json:"round"`        // 当前轮次
	RoundTime    float64 `json:"roundTime"`    // 本轮用时（秒）
	TotalTime    float64 `json:"totalTime"`    // 累计总时间（秒）
	Saved        bool    `json:"saved"`        // 是否保存到数据库（仅第3轮）
	GameComplete bool    `json:"gameComplete"` // 是否完成全部三轮
}

/********* Handlers *********/

// GameMapStart godoc
// @Summary     开始地图游戏
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Param       body  body      startMapGameReq  true  "请求参数"
// @Success     200   {object}  startMapGameResp  "响应数据"
// @Router      /api/game/map/start [post]
func GameMapStart(c *gin.Context) { //开始游戏
	// 不用请求参数-因为用户一点击按钮就开始游戏了

	uid := c.GetUint("user_id")
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	mapGame.mu.Lock()
	// 取/建玩家状态
	p, ok := mapGame.Players[uid] // 取玩家状态
	if !ok {
		p = init_MapGamePlayer() // 如果玩家不存在就初始化表
		mapGame.Players[uid] = p // 保存玩家状态-全局变量
	}

	// 根据当前轮次设置难度
	difficulty := getDifficultyForRound(p.Round) // 根据当前轮次设置难度-这个函数是限定的
	p.Difficulty = difficulty                    // 保存难度-全局变量
	mapGame.mu.Unlock()                          // 解锁-以求用户进行按钮或者触发
	// 生成地图
	size := game_rounds[difficulty]
	arr := array_init(size)

	// 生成起点
	startPoint := P{}
	startPoint.X, startPoint.Y = start_index(arr)
	arr[startPoint.X][startPoint.Y] = '+'

	// 简化路径生成 - 直接生成足够的路径
	go_next(arr, startPoint, size, difficulty)

	// 找到终点
	endPoint, currentDistance := end_index(arr, startPoint)
	arr[endPoint.X][endPoint.Y] = 'x' //终点标记为x

	// 调试：统计地图内容和打印
	pathCount := 0
	wallCount := 0
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			if arr[i][j] == 'o' {
				pathCount++
			} else if arr[i][j] == '#' {
				wallCount++
			}
		}
	}
	// 更新玩家状态
	mapGame.mu.Lock()
	p.MapData = arr               // 保存当前的地图数据-全局变量
	p.StartPoint = startPoint     // 保存起点-全局变量
	p.EndPoint = endPoint         // 保存终点-全局变量
	p.RoundStartTime = time.Now() // 保存当前轮开始时间-全局变量
	p.IsRoundCompleted = false    // 保存当前轮是否完成-全局变量
	mapGame.mu.Unlock()           // 解锁-以求用户进行按钮或者触发
	// 返回响应
	rows := make([]string, size)
	for i := 0; i < size; i++ {
		rows[i] = string(arr[i]) // []byte → string（UTF-8）
	}
	resp := startMapGameResp{
		Message:         fmt.Sprintf("地图游戏第 %d 轮已开始！难度：%s，地图大小：%dx%d", p.Round, getDifficultyName(difficulty), size, size),
		Round:           p.Round,
		Difficulty:      difficulty,
		Size:            size,
		MapData:         rows, // ← 用 rows
		StartPoint:      startPoint,
		EndPoint:        endPoint,
		CurrentDistance: currentDistance,
		TotalTime:       p.TotalTime,
	}
	c.JSON(http.StatusOK, resp)
}

// GameMapComplete godoc
// @Summary     完成地图游戏
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  completeMapGameResp  "响应数据"
// @Router      /api/game/map/complete [post]
func GameMapComplete(c *gin.Context) {
	uid := c.GetUint("user_id")
	uname := c.GetString("username")
	if uid == 0 || uname == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	mapGame.mu.Lock()
	player, ok := mapGame.Players[uid]  // 取玩家状态
	if !ok || player.IsRoundCompleted { //如果玩家不存在就初始化表
		mapGame.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "user not found or no active game found or round already completed"})
		return
	}

	// 计算本轮用时（秒）
	roundTime := time.Since(player.RoundStartTime).Seconds() // 按秒来计算
	player.TotalTime += roundTime                            // 累计总时间
	player.IsRoundCompleted = true                           //上一轮完成

	var resp completeMapGameResp

	if player.Round == 3 {
		// 第3轮完成，游戏结束
		finalTime := player.TotalTime
		saved := true
		if err := saveMapGameScore(uid, uname, finalTime); err != nil { // 显示MySql再是redis排行榜
			log.L().Error("saveMapGameScore failed", zap.Error(err))
			saved = false // 保存失败
		}
		log.L().Info("A user's data has saved!!!\n")
		resp = completeMapGameResp{
			Message:      fmt.Sprintf("恭喜完成第 3 轮！本轮用时：%.2f 秒。三轮总用时：%.2f 秒。已为您开启新的一局。", roundTime, finalTime),
			Round:        player.Round,
			RoundTime:    roundTime, //本轮时间
			TotalTime:    finalTime,
			Saved:        saved, //
			GameComplete: true,
		}

		// 重置玩家状态，开启新的一局
		mapGame.Players[uid] = init_MapGamePlayer()
		mapGame.mu.Unlock()
		c.JSON(http.StatusOK, resp)
		return
	}

	// 未完成全部三轮，进入下一轮
	nextRound := player.Round + 1
	nextDifficulty := getDifficultyForRound(nextRound) //标明难度

	resp = completeMapGameResp{
		Message:      fmt.Sprintf("恭喜完成第 %d 轮！本轮用时：%.2f 秒。进入第 %d 轮（%s难度）。当前总用时：%.2f 秒。", player.Round, roundTime, nextRound, getDifficultyName(nextDifficulty), player.TotalTime), // 返回消息
		Round:        player.Round,
		RoundTime:    roundTime,
		TotalTime:    player.TotalTime,
		Saved:        false,
		GameComplete: false,
	}

	// 更新玩家状态进入下一轮
	player.Round = nextRound
	player.IsRoundCompleted = false
	// 注意：不重置RoundStartTime，等下次调用GameMapStart时再设置

	mapGame.mu.Unlock()
	c.JSON(http.StatusOK, resp)
}

// GameMapReset godoc
// @Summary     重置地图游戏状态
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  gin.H{"message": "当前用户的地图游戏状态已重置，可以重新开始游戏", "reset":true}  "响应数据"
// @Router      /api/game/map/reset [post]
func GameMapReset(c *gin.Context) { // 重置按钮-清空当前用户的游戏状态
	uid := c.GetUint("user_id")
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	mapGame.mu.Lock()
	delete(mapGame.Players, uid) // 只删除当前用户的游戏状态
	mapGame.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"message": "当前用户的地图游戏状态已重置，可以重新开始游戏",
		"reset": true})
}

/********* 地图生成辅助函数 *********/
// 初始化地图
func array_init(size int) [][]byte {
	arr := make([][]byte, size)
	for i := 0; i < size; i++ {
		arr[i] = make([]byte, size)
	}
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			arr[i][j] = '#'
		}
	}
	return arr
}

// 打印地图（调试用）
func print_array(arr [][]byte) {
	for i := 0; i < len(arr); i++ {
		for j := 0; j < len(arr[i]); j++ {
			fmt.Printf("%c ", arr[i][j])
		}
		fmt.Printf("\n")
	}
}

// 生成起点
func start_index(arr [][]byte) (int, int) {
	row := len(arr)
	col := len(arr[0])
	x := rand.Intn(row)
	y := rand.Intn(col)
	return x, y
}

// 递归生成路径 - 修改算法确保生成足够路径
func go_next(arr [][]byte, start_point P, step int, step_rand int) {
	if step <= 0 {
		return
	}
	for i := 0; i < len(dir); i++ {
		newX := start_point.X + dir[i][0]
		newY := start_point.Y + dir[i][1]
		if newX >= 0 && newX < len(arr) && newY >= 0 && newY < len(arr[0]) {
			if arr[newX][newY] == '#' {
				randnum := rand.Intn(2) //
				if randnum > 0 || step_rand > 0 {
					arr[newX][newY] = 'o'
					go_next(arr, P{newX, newY}, step-1, step_rand-1)
				}
			}
		}
		// 不满足条件则跳出
	}
	// 最后也是跳出
}

// BFS 找到最远的终点
func end_index(arr [][]byte, start_point P) (end_point P, far int) {
	row, col := len(arr), len(arr[0])
	far = 0
	const p_dist = -1
	distMap := make([][]int, row)
	for i := 0; i < row; i++ {
		distMap[i] = make([]int, col)
		for j := range distMap[i] {
			distMap[i][j] = p_dist
		}
	}
	queue := make([]P, 0)
	queue_push := func(p P, distance int) {
		distMap[p.X][p.Y] = distance
		queue = append(queue, p)
	}
	queue_push(start_point, 0)

	for head := 0; head < len(queue); head++ {
		p := queue[head]
		d := distMap[p.X][p.Y]
		if d > far {
			far = d
			end_point = p
		}
		for i := 0; i < len(dir); i++ {
			newX := p.X + dir[i][0]
			newY := p.Y + dir[i][1]
			if newX >= 0 && newX < row && newY >= 0 && newY < col {
				if distMap[newX][newY] != p_dist {
					continue
				}
				if arr[newX][newY] == 'o' || arr[newX][newY] == '+' {
					distMap[newX][newY] = d + 1
					queue_push(P{newX, newY}, d+1)
				}
			}
		}
	}
	return end_point, far
}

/********* DB 辅助 *********/

// 保存地图游戏完成时间
func saveMapGameScore(uid uint, username string, timeSeconds float64) (err error) {
	if uid == 0 || username == "" || timeSeconds <= 0 {
		return fmt.Errorf("invalid save params: uid=%d username='%s' time=%.3f", uid, username, timeSeconds)
	}
	// 开始数据库事务
	tx := global.DB.Begin()
	if err = tx.Error; err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit().Error
		}
	}()

	// 1) 统计该用户已有记录数
	var cnt int64
	if err = tx.Model(&models.Game_Map_Time{}).
		Where("user_id = ?", uid).
		Count(&cnt).Error; err != nil { //统计数
		return
	}

	// 2) 未达上限，直接新增
	if cnt < map_users_number { //数据库用户的上线人数
		rec := models.Game_Map_Time{UserID: uid, Score: timeSeconds, UserName: username}
		if err = tx.Create(&rec).Error; err != nil {
			return
		}
		// 更新 Redis 排行榜
		_ = updateTop10FastestAfterDB(uid, username, timeSeconds)
		return
	}

	// 3) 达上限：找到用时最长的记录并更新 - slowest为最慢的用户
	var slowest models.Game_Map_Time
	if err = tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", uid).
		Order("created_at ASC, id ASC").First(&slowest).Error; err != nil { // 升序排序-这里按照创建时间升序

		if errors.Is(err, gorm.ErrRecordNotFound) {
			rec := models.Game_Map_Time{UserID: uid, Score: timeSeconds, UserName: username}
			err = tx.Create(&rec).Error
			if err == nil {
				_ = updateTop10FastestAfterDB(uid, username, timeSeconds)
			}
			return
		}
		return
	}

	now := time.Now()
	err = tx.Model(&slowest).Updates(map[string]interface{}{ // 更新最慢的用户表中的数据
		"score":      timeSeconds,
		"created_at": now,
		"username":   username,
	}).Error
	if err != nil {
		return
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	_ = updateTop10FastestAfterDB(uid, username, timeSeconds)

	return
}

/********* Redis 排行榜 *********/

// Lua 脚本更新排行榜（用时越短越好，分数越低越好） - 执行脚本
var luaUpdateTop10Fastest = redis.NewScript(`  
local key     = KEYS[1]
local hname   = KEYS[2]
local member  = ARGV[1]
local score   = tonumber(ARGV[2])
local topK    = tonumber(ARGV[3])
local uname   = ARGV[4]

local cur = redis.call('ZSCORE', key, member)
if (not cur) or (score < tonumber(cur)) then 
  redis.call('ZADD', key, score, member)
end

local n = redis.call('ZCARD', key)
if n > topK then
  redis.call('ZREMRANGEBYRANK', key, topK, -1)
end

if uname and uname ~= '' then
  redis.call('HSET', hname, member, uname)
end
return 1
`)

// 更新地图游戏排行榜（用时越短分数越低，排名越靠前）
func updateTop10FastestAfterDB(userID uint, username string, timeSeconds float64) error {
	if userID == 0 || timeSeconds < 0 || global.RedisDB == nil {
		return nil
	}
	member := strconv.FormatUint(uint64(userID), 10)
	_, err := luaUpdateTop10Fastest.Run(global.RedisDB,
		[]string{config.RedisKeyTop10FastestMap, config.RedisKeyUsernames}, // key2和key2
		member,      // ARGV[1]
		timeSeconds, // ARGV[2] - 用时（秒）
		10,          // ARGV[3] TopK=10
		username,    // ARGV[4]
	).Result()
	return err
}
