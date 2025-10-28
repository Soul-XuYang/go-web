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
	"project/utils"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const users_number = 10

type GamePlayer struct {
	Round      int // 1..3
	Attempts   int // 当前轮已用次数
	TotalScore int // 当前这局三轮累计分
	Target     int // 当前轮目标数
}

type gameState struct {
	Players         map[uint]*GamePlayer
	BaseMaxAttempts int // 第1轮的最大次数，后续轮次依次 -1
	mu              sync.Mutex
}

var game = &gameState{
	Players:         make(map[uint]*GamePlayer),
	BaseMaxAttempts: 9,
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func (g *gameState) allowedAttemptsFor(round int) int {
	a := g.BaseMaxAttempts - (round - 1)
	if a < 1 {
		return 1
	}
	return a
}

func (g *gameState) newTarget() int {
	return rand.Intn(100) + 1
}

/********* DTO *********/
type guessReq struct {
	Num int `json:"number" binding:"required,min=1,max=100"`
}

type guessResp struct {
	Message    string `json:"message"`
	Status     string `json:"status"` // low/high/correct
	Attempts   int    `json:"attempts"`
	Remaining  int    `json:"remaining"`
	TotalScore int    `json:"totalScore"`
	Round      int    `json:"round"`
}

/********* Handlers *********/

func init_GamePlayer() *GamePlayer {
	return &GamePlayer{
		Round:      1,
		Attempts:   0,
		TotalScore: 0,
		Target:     game.newTarget(),
	}
}

// GameGuess godoc
// @Summary     猜数字游戏
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Param       body  body      guessReq  true  "请求参数"
// @Success     200   {object}  guessResp  "响应数据"
// @Router      /api/gameguess [post]
func GameGuess(c *gin.Context) { // 注意这里游戏表的用户id和用户名称都是从上下文中获取的，本身id就是外键就是共生共存的
	var in guessReq
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	uid := c.GetUint("user_id")
	uname := c.GetString("username") // uname是我上下文传来的
	if uid == 0 || uname == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var resp guessResp

	game.mu.Lock()
	// 取/建玩家状态
	p, ok := game.Players[uid]
	if !ok {
		p = init_GamePlayer()
		game.Players[uid] = p
	}
	allowed := game.allowedAttemptsFor(p.Round)

	// 本次尝试
	p.Attempts++

	switch {
	case in.Num < p.Target:
		resp.Status = "low"
		remain := utils.MaxInt(0, allowed-p.Attempts)
		resp.Message = fmt.Sprintf("第 %d 轮：数字太小了！你还有 %d 次机会。", p.Round, remain)

		if p.Attempts >= allowed {
			roundEndMessage := fmt.Sprintf("第 %d 轮已用完 %d 次机会。答案是 %d。", p.Round, allowed, p.Target)
			if p.Round == 3 {
				finalScore := p.TotalScore
				// 写入 DB + 更新 Redis 排行（失败也会写分为当前总分）
				if err := saveGameScore(uid, uname, finalScore); err != nil {
					log.L().Error("saveGameScore failed", zap.Error(err))
				}

				resp.Message = fmt.Sprintf("%s 三轮结束！本局总分：%d。已为你开启新的一局。", roundEndMessage, finalScore)
				resp.Attempts = p.Attempts
				resp.Remaining = 0
				resp.TotalScore = finalScore
				resp.Round = p.Round

				game.Players[uid] = init_GamePlayer()
				game.mu.Unlock()
				c.JSON(http.StatusOK, resp)
				return
			}

			// 进入下一轮
			resp.Message = fmt.Sprintf("%s 进入第 %d 轮（可用次数：%d）。", roundEndMessage, p.Round+1, game.allowedAttemptsFor(p.Round+1))
			resp.Attempts = p.Attempts
			resp.Remaining = 0
			resp.TotalScore = p.TotalScore
			resp.Round = p.Round

			p.Round++
			p.Attempts = 0
			p.Target = game.newTarget()

			game.mu.Unlock()
			c.JSON(http.StatusOK, resp)
			return
		}

	case in.Num > p.Target:
		resp.Status = "high"
		remain := utils.MaxInt(0, allowed-p.Attempts)
		resp.Message = fmt.Sprintf("第 %d 轮：数字太大了！你还有 %d 次机会。", p.Round, remain)

		if p.Attempts >= allowed {
			roundEndMessage := fmt.Sprintf("第 %d 轮已用完 %d 次机会。答案是 %d。", p.Round, allowed, p.Target)
			if p.Round == 3 {
				finalScore := p.TotalScore
				if err := saveGameScore(uid, uname, finalScore); err != nil {
					log.L().Error("saveGameScore failed", zap.Error(err))
				}

				resp.Message = fmt.Sprintf("%s 三轮结束！本局总分：%d。已为你开启新的一局。", roundEndMessage, finalScore)
				resp.Attempts = p.Attempts
				resp.Remaining = 0
				resp.TotalScore = finalScore
				resp.Round = p.Round

				game.Players[uid] = init_GamePlayer()
				game.mu.Unlock()
				c.JSON(http.StatusOK, resp)
				return
			}

			resp.Message = fmt.Sprintf("%s 进入第 %d 轮（可用次数：%d）。", roundEndMessage, p.Round+1, game.allowedAttemptsFor(p.Round+1))
			resp.Attempts = p.Attempts
			resp.Remaining = 0
			resp.TotalScore = p.TotalScore
			resp.Round = p.Round

			p.Round++
			p.Attempts = 0
			p.Target = game.newTarget()

			game.mu.Unlock()
			c.JSON(http.StatusOK, resp)
			return
		}

	default:
		// 猜中
		resp.Status = "correct"
		inc := allowed - p.Attempts + 1
		if inc < 0 {
			inc = 0
		}
		p.TotalScore += inc * p.Round

		resp.Attempts = p.Attempts
		resp.Remaining = utils.MaxInt(0, allowed-p.Attempts)
		resp.TotalScore = p.TotalScore

		if p.Round == 3 {
			finalScore := p.TotalScore
			if err := saveGameScore(uid, uname, finalScore); err != nil {
				log.L().Error("saveGameScore failed", zap.Error(err))
			}

			resp.Message = fmt.Sprintf("第 3 轮：恭喜 %s 猜对！本轮用 %d 次，三轮总分：%d。已为你开启新的一局。", uname, p.Attempts, finalScore)
			resp.TotalScore = p.TotalScore
			resp.Round = p.Round

			game.Players[uid] = init_GamePlayer()
			game.mu.Unlock()
			c.JSON(http.StatusOK, resp)
			return
		}

		// 进入下一轮
		resp.Message = fmt.Sprintf("第 %d 轮：恭喜 %s 猜对！本轮共用 %d 次，当前总分：%d。进入第 %d 轮（可用次数：%d）。",
			p.Round, uname, resp.Attempts, p.TotalScore, p.Round+1, game.allowedAttemptsFor(p.Round+1))

		p.Round++
		p.Attempts = 0
		p.Target = game.newTarget()
		resp.Round = p.Round

		game.mu.Unlock()
		c.JSON(http.StatusOK, resp)
		return
	}

	// 未猜中且仍有机会，常规返回
	resp.Attempts = p.Attempts
	resp.Remaining = utils.MaxInt(0, allowed-p.Attempts)
	resp.TotalScore = p.TotalScore
	game.mu.Unlock()

	c.JSON(http.StatusOK, resp)
}

func GameGuess_Reset(c *gin.Context) {
	game.mu.Lock()
	game.Players = make(map[uint]*GamePlayer)
	game.BaseMaxAttempts = 9
	game.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "游戏已重置（清空所有玩家进行中的三轮局）。"})
}

/********* DB 保存数据 *********/

// 统一的保存函数：写入 MySQL（表 models.Game_Guess_Score），并在成功后更新 Redis 排行榜
func saveGameScore(uid uint, username string, score int) (err error) {
	if uid == 0 {
		return nil
	}

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
	if err = tx.Model(&models.Game_Guess_Score{}).
		Where("user_id = ?", uid).
		Count(&cnt).Error; err != nil {
		return
	}

	// 2) 未达上限，直接新增
	if cnt < users_number {
		rec := models.Game_Guess_Score{UserID: uid, Score: score, UserName: username}
		if err = tx.Create(&rec).Error; err != nil {
			return
		}
		// 新增成功后更新 redis 排行（事务提交完毕后也会执行，因为 defer commit 在后）
		// 为避免事务未提交即更新 redis，先尝试更新（若想更严格可在事务外再调用）
		_ = updateTop10BestAfterDB(uid, username, score)
		return
	}

	// 3) 达上限：锁定且最早的记录并更新（就地复用）
	var oldest models.Game_Guess_Score
	if err = tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", uid).
		Order("created_at ASC, id ASC").First(&oldest).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			rec := models.Game_Guess_Score{UserID: uid, Score: score, UserName: username}
			err = tx.Create(&rec).Error
			if err == nil {
				_ = updateTop10BestAfterDB(uid, username, score)
			}
			return
		}
		return
	}

	now := time.Now()
	err = tx.Model(&oldest).Updates(map[string]interface{}{
		"score":      score,
		"created_at": now,
		"username":   username,
	}).Error
	if err != nil {
		return
	}

	// 更新 redis 排行
	_ = updateTop10BestAfterDB(uid, username, score)

	return
}


//--------------------------------------------------
// lua 脚本，用于更新排行榜（保留 topK，并保存用户名哈希）
// 这里获取当前玩家再排行榜的分数，不沉溺在或者分数更高则更新玩家的分数
// 删除多余的排名，确保只保留 topK
var luaUpdateTop10Best = redis.NewScript(`
local key     = KEYS[1]
local hname   = KEYS[2]
local member  = ARGV[1]
local score   = tonumber(ARGV[2])
local topK    = tonumber(ARGV[3])
local uname   = ARGV[4]

local cur = redis.call('ZSCORE', key, member)
if (not cur) or (score > tonumber(cur)) then 
  redis.call('ZADD', key, score, member)
end

local n = redis.call('ZCARD', key)
if n > topK then
  redis.call('ZREMRANGEBYRANK', key, 0, n - topK - 1)
end

if uname and uname ~= '' then
  redis.call('HSET', hname, member, uname)
end
return 1
`)

func updateTop10BestAfterDB(userID uint, username string, finalScore int) error {
	if userID == 0 || finalScore < 0 || global.RedisDB == nil {
		return nil
	}
	member := strconv.FormatUint(uint64(userID), 10) //这里的member是字符串类型，传入的id是分数表中的userID
	_, err := luaUpdateTop10Best.Run(global.RedisDB,
		[]string{config.RedisKeyTop10Best, config.RedisKeyUsernames},
		member,     // ARGV[1]
		finalScore, // ARGV[2]
		10,         // ARGV[3] TopK=10
		username,   // ARGV[4]
	).Result()
	return err
}


