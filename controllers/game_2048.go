package controllers

import (
	"fmt"
	"net/http"
	"project/global"
	"project/models"
	"sync"
	"project/config"

	"github.com/gin-gonic/gin"
)

const game2048_number = 10
var (
    game2048Mutex sync.Mutex
)
// 场景用户A和用户B同时在玩2048游戏 用户A得分1024，用户B得分1024 两个用户几乎同时点击保存分数
// 用户A的请求进来，创建了自己的锁 mu_A 用户B的请求进来，创建了自己的锁 mu_B 这两个锁是不同的对象，无法实现互斥访问
type Game2048SaveRequest struct {
	Score int `json:"score" binding:"required,min=0"`
}

// Game2048SaveScore 保存2048游戏分数
// @Summary 保存2048游戏分数
// @Description 保存用户的2048游戏分数到数据库
// @Tags Game
// @Accept json
// @Produce json
// @Param score body Game2048SaveRequest true "游戏分数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/game/2048/save [post]
func Game2048SaveScore(c *gin.Context) {
	userID := c.GetUint("user_id")
	username := c.GetString("username")
	if userID == 0 || username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "this User has no permission"})
		return
	}
    
	
	var req Game2048SaveRequest //获取保存请求
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	if req.Score <= 0 || req.Score > 2048 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invaild Score"})
		return
	}
    game2048Mutex.Lock()
	defer game2048Mutex.Unlock()
	// Save to database
	err := save2048Score(userID, username, req.Score)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Score saved successfully",
		"score":   req.Score,
	})
}

// save2048Score 保存2048游戏分数到数据库
func save2048Score(uid uint, username string, score int) error {
	if uid == 0 || username == "" || score <= 0 {
		return fmt.Errorf("invalid params: uid=%d username='%s' score=%d", uid, username, score)
	}

	// 开始构建数据库事务
	tx := global.DB.Begin()
	if err := tx.Error; err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// 统计该用户已有记录数
	var cnt int64
	if err := tx.Model(&models.Game_2048_Score{}).
		Where("user_id = ?", uid).
		Count(&cnt).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 创建新记录
	newRecord := models.Game_2048_Score{
		UserID:   uid,
		UserName: username,
		Score:    score,
	}

	if err := tx.Create(&newRecord).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 如果超过10条，删除最旧的
	if cnt >= game2048_number {
		deleteCount := cnt - 4 // 保留最新的10条

		// 查找最旧的记录ID
		var oldIDs []uint
		if err := tx.Model(&models.Game_2048_Score{}).
			Select("id").
			Where("user_id = ?", uid).
			Order("created_at ASC").
			Limit(int(deleteCount)).
			Pluck("id", &oldIDs).Error; err != nil {
			tx.Rollback()
			return err
		}

		// 删除这些记录
		if len(oldIDs) > 0 {
			if err := tx.Where("id IN ?", oldIDs).Delete(&models.Game_2048_Score{}).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	// 上述的情况都已满足-无论是缓存满不满都要加入到redis里
    _ = updateTop10BestAfterDB(config.RedisKeyTop10Game2048,config.RedisKeyGameUsernames,uid, username, score)
	// 提交事务
	return tx.Commit().Error
}
