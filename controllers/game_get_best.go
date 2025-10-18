package controllers

import (
	"net/http"
	"project/config"
	"project/global"
	"project/models"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
)

// 这里是从redis中读取某个用户的最佳成绩和排名
func myBestAndRank(zsetKey string, uid uint) (best int, rank int, err error) {
	if uid == 0 || zsetKey == "" {
		return 0, 0, nil
	}

	// 先尝试从 Redis 读（只对 Top10 有效）
	if global.RedisDB != nil {
		member := strconv.FormatUint(uint64(uid), 10)

		// best
		sc, e1 := global.RedisDB.ZScore(zsetKey, member).Result()
		if e1 != nil && e1 != redis.Nil {
			return 0, 0, e1
		}
		if e1 == nil { // 在 Top10 里
			best = int(sc)
			// rank（0-based -> 1-based）
			r0, e2 := global.RedisDB.ZRevRank(zsetKey, member).Result()
			if e2 != nil && e2 != redis.Nil {
				return best, 0, e2
			}
			if e2 == nil {
				return best, int(r0) + 1, nil // 在 Top10 里返回redis的排名数据
			}
			// 理论上走不到这里
		}
	}

	// 不在 Top10：直接回落到 MySQL 查历史最佳
	best, err = queryBestFromDB(uid)
	if err != nil {
		return 0, 0, err
	}
	// 未上榜 -> rank = 0
	return best, 0, nil
}

// --------- 接口：GET /api/game/leaderboard/me?game=guess_game ---------
func GameLeaderboardMe(c *gin.Context) {
	// 首先读取用户信息
	uid := c.GetUint("user_id")
	uname := c.GetString("username")
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 游戏代号：不传则默认 guess_game（与你前端常量一致）-这里是根据前端传来的游戏名称来决定读取哪个排行榜
	gameCode := c.DefaultQuery("game", "guess_game") // key是game
	zkey, ok := boards[gameCode]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game"})
		return
	}

	// 如果ok则读取我的 best & rank
	best, rank, err := myBestAndRank(zkey, uid) // 根据当前的id读取数据
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read leaderboard failed"})
		return
	}

	// 顺手把用户名写入统一 Hash（幂等；避免老数据没有名字）-防御操作
	if global.RedisDB != nil && uname != "" {
		_ = global.RedisDB.HSet(config.RedisKeyUsernames, strconv.FormatUint(uint64(uid), 10), uname).Err()
	}
	// 这里是返回对应的数据
	c.JSON(http.StatusOK, gin.H{
		"game":     gameCode,
		"user_id":  uid,
		"username": uname,
		"best":     best, // 历史最佳（来自该游戏的 ZSET）
		"rank":     rank, // 当前排名（未上榜=0）
	})
}

// 这里是从数据库中查询某个用户的最佳成绩
func queryBestFromDB(uid uint) (int, error) {
	if uid == 0 {
		return 0, nil
	}
	var best int
	err := global.DB.
		Model(&models.Game_Guess_Score{}).
		Where("user_id = ?", uid).
		Select("MAX(score) AS best").
		Scan(&best).Error
	if err != nil {
		return 0, err
	}
	return best, nil
}
