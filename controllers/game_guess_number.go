package controllers

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
	"errors"

	"github.com/gin-gonic/gin"
    "gorm.io/gorm"
	"gorm.io/gorm/clause"
	"project/global"
	"project/models"
	"project/utils"
)
const users_number = 5

type GamePlayer struct {
	// 当前这局（三轮制）内的状态
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
	Players:         make(map[uint]*GamePlayer),  //哈希表来判断呢
	BaseMaxAttempts: 9,
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func (g *gameState) allowedAttemptsFor(round int) int { //按照当前轮次递减
	a := g.BaseMaxAttempts - (round - 1)
	if a < 1 {
		return 1
	}
	return a
}

func (g *gameState) newTarget() int { // 创建随机数
	return rand.Intn(100) + 1
}

/********* DTO *********/
type guessReq struct {
	Num int `json:"number" binding:"required,min=1,max=100"`
}

// 响应的DTO
type guessResp struct {
	Message    string `json:"message"`
	Status     string `json:"status"` // low/high/correct
	Attempts   int    `json:"attempts"`
	Remaining  int    `json:"remaining"`
	TotalScore int    `json:"totalScore"` // 既是当前分数也是最终三次后的总分数
	Round int  `json:"round"`
}

/********* Handlers *********/

func init_GamePlayer() *GamePlayer {
    return &GamePlayer{
        Round:      1,
        Attempts:   0,
        TotalScore: 0,
        Target:     game.newTarget(), // 假设 game 是一个可访问的实例
    }
}
// 猜数字（三轮制，次数递减）
func GameGuess(c *gin.Context) {
	var in guessReq
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	uid := c.GetUint("user_id")
	uname := c.GetString("username")
	if uid == 0 || uname == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var resp guessResp

	game.mu.Lock()
	// 取/建玩家状态
	p, ok := game.Players[uid] // 最初ok为false创建玩家状态-这里用哈希表来判断确认
	if !ok {
		p = init_GamePlayer() // 初始化玩家状态
		game.Players[uid] = p // 存入哈希表判断是否有这个玩家
	}
	allowed := game.allowedAttemptsFor(p.Round) // 按每轮的一个次数计算

	// 本次尝试
	p.Attempts++

	switch {
	case in.Num < p.Target: // 太小了
		resp.Status = "low"
		remain := utils.MaxInt(0, allowed-p.Attempts) // 当前的尝试机会-已用次数
		resp.Message = fmt.Sprintf("第 %d 轮：数字太小了！你还有 %d 次机会。", p.Round, remain)
		// 这里是当前轮次的使用机会
		if p.Attempts >= allowed {
			//这里是用完所有机会-游戏结束
			roundEndMessage := fmt.Sprintf("第 %d 轮已用完 %d 次机会。答案是 %d。", p.Round, allowed, p.Target)
			if p.Round == 3 {
				// 三轮结束 —— 写入数据库并开新局
				finalScore := p.TotalScore // 未加分（本轮失败）
				_ = saveGameScore(uid, finalScore)

				resp.Message = fmt.Sprintf("%s 三轮结束！本局总分：%d。已为你开启新的一局。", roundEndMessage, finalScore)
				// 组织返回的统计（本轮）
				resp.Attempts = p.Attempts
				resp.Remaining = 0
				resp.TotalScore = finalScore
				resp.Round = p.Round

				// 这里开始重置为新局
				game.Players[uid] = init_GamePlayer()

				game.mu.Unlock()            // 解锁
				c.JSON(http.StatusOK, resp) // 返回响应
				return
			}

			// 当前轮次用完并那就进入到下一轮
			resp.Message = fmt.Sprintf("%s 进入第 %d 轮（可用次数：%d）。", roundEndMessage, p.Round+1, game.allowedAttemptsFor(p.Round+1))
			resp.Attempts = p.Attempts
			resp.Remaining = 0
			resp.TotalScore = p.TotalScore //本轮失败，不加分
            resp.Round = p.Round
			// 切换轮次
			p.Round++
			p.Attempts = 0
			p.Target = game.newTarget()

			game.mu.Unlock()            //解锁
			c.JSON(http.StatusOK, resp) //返回此刻的信息
			return
		}

	case in.Num > p.Target:
		resp.Status = "high"
		remain := utils.MaxInt(0, allowed-p.Attempts)
		resp.Message = fmt.Sprintf("第 %d 轮：数字太大了！你还有 %d 次机会。", p.Round, remain)

		if p.Attempts >= allowed {
			// 本轮失败，轮次结束
			roundEndMessage := fmt.Sprintf("第 %d 轮已用完 %d 次机会。答案是 %d。", p.Round, allowed, p.Target)
			if p.Round == 3 {
				finalScore := p.TotalScore
				_ = saveGameScore(uid, finalScore)

				resp.Message = fmt.Sprintf("%s 三轮结束！本局总分：%d。已为你开启新的一局。", roundEndMessage, finalScore)
				resp.Attempts = p.Attempts
				resp.Remaining = 0
				resp.TotalScore = finalScore
                resp.Round = p.Round

				game.Players[uid] = init_GamePlayer()

				game.mu.Unlock() // 解锁
				c.JSON(http.StatusOK, resp)
				return
			}

			//下一轮
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
		// 得分：当轮可用次数 - 已用次数 + 1
		inc := allowed - p.Attempts + 1 //运行的次数反过来
		if inc < 0 {
			inc = 0
		}
		p.TotalScore += inc * p.Round

		// 组织当前轮返回统计（在切轮前）
		resp.Attempts = p.Attempts
		resp.Remaining = utils.MaxInt(0, allowed-p.Attempts) //剩余次数
		resp.TotalScore = p.TotalScore

		if p.Round == 3 {
			// 三轮完成，写库并开新局
			finalScore := p.TotalScore
			_ = saveGameScore(uid, finalScore)
          
			resp.Message = fmt.Sprintf("第 3 轮：恭喜 %s 猜对！本轮用 %d 次，三轮总分：%d。已为你开启新的一局。", uname, p.Attempts, finalScore)
			resp.TotalScore = p.TotalScore
            resp.Round = p.Round

			// 重置为新局
            game.Players[uid] = init_GamePlayer() // 这里是重置玩家状态
			game.mu.Unlock()
			c.JSON(http.StatusOK, resp)
			return
		}

		// 非第3轮，进入下一轮
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

// 重置游戏（清空所有玩家当前局）
func GameGuess_Reset(c *gin.Context) {
	game.mu.Lock()
	game.Players = make(map[uint]*GamePlayer)
	game.BaseMaxAttempts = 9
	game.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "游戏已重置（清空所有玩家进行中的三轮局）。"})
}

/********* DB 辅助 *********/

func saveGameScore(userID uint, score int) (err error) {
	if userID == 0 {
		return nil
	}

	tx := global.DB.Begin()  // 开启事务
	if err = tx.Error; err != nil {
		return err
	}

	// 统一提交/回滚 + panic 保护
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // 继续把 panic 抛出去，按需处理
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit().Error
		}
	}()

	// 1) 统计该用户已有记录数-传入cnt数据
	var cnt int64
	if err = tx.Model(&models.Game_Guess_Score{}).  
		Where("user_id = ?", userID).
		Count(&cnt).Error; err != nil {
		return  
	}

	// 2) 未达上限，直接新增
	if cnt < users_number {
		rec := models.Game_Guess_Score{UserID: userID, Score: score}
		err = tx.Create(&rec).Error
		return
	}

	// 3) 达上限：锁定最早一条并就地复用
	var oldest models.Game_Guess_Score
	if err = tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		Order("created_at ASC, id ASC").  // 按照创建时间升序排序-获取第一个，这里就是最早的时间
		First(&oldest).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 理论少见：回退到新增
			rec := models.Game_Guess_Score{UserID: userID, Score: score}
			err = tx.Create(&rec).Error
			return
		}
		return
	}

	now := time.Now()
	err = tx.Model(&oldest).UpdateColumns(map[string]interface{}{ // 更新列
		"score":      score,
		"created_at": now,
		// "updated_at": now, // 如需同步更新
	}).Error

	return
}



