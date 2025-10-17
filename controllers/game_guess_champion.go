package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"project/global"
	"project/models"
)

// 返回体-DTO
type championResp struct {
	ID       uint   `json:"id"`
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	Score    int    `json:"score"`
	Rounds   int    `json:"rounds"`
	Created  int64  `json:"created"`
}

// GameChampion godoc
// @Summary      猜数字：全体用户第一名（全服冠军）
// @Tags         Game
// @Security     Bearer
// @Produce      json
// @Success      200 {object} controllers.championResp
// @Success      204 {object} nil "暂无数据"
// @Failure      500 {object} map[string]string
// @Router       /game/scores/champion [get]
func GameChampion(c *gin.Context) {
	// 可选：鉴权严格一些
	// uid := c.GetUint("user_id")
	// if uid == 0 { c.JSON(401, gin.H{"error":"unauthorized"}); return }

	var rows[] models.Game_Guess_Score
   err := global.DB.Raw(`
    SELECT g.* FROM game_guess_scores g
    JOIN (SELECT user_id, MAX(score) max_score
    FROM game_guess_scores GROUP BY user_id) ms
    ON g.user_id = ms.user_id AND g.score = ms.max_score
    ORDER BY g.score DESC, g.created_at DESC, g.id DESC
    LIMIT 1 
    `).Scan(&rows).Error  // 每个用户只有1名
    //错误判断
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNoContent) // 204：没有数据
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
     
    var champions []championResp
    //设置数据
    for _, row := range rows {
        champion := championResp{ // 构建结构体变量
            ID:       row.ID,
            UserID:   row.UserID,
            Username: row.User.Username,
            Score:    row.Score,
            Created:  row.CreatedAt.Unix(),
        }
        champions = append(champions, champion)
    }
    c.JSON(http.StatusOK, champions)
}
