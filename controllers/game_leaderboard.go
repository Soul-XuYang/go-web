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

var (
	// 请求限制器
	leaderboardLimiter = make(chan struct{}, 1000) // 限制并发1000
)

func exit_thread() { <-leaderboardLimiter }

// 排行榜设置界面
// 这里是从redis中读取某个用户的最佳成绩和排名
// 分数排行设置isLowerBetter: true表示分数越低越好（如用时），false表示分数越高越好（如得分）
func myBestAndRank(zsetKey string, uid uint, isLowerBetter bool) (best int, rank int, err error) { //这里按照默认false为高排序
	if uid == 0 || zsetKey == "" {
		return 0, 0, nil
	}

	// 先尝试从当前缓存Redis来读取数据
	if global.RedisDB != nil {
		member := strconv.FormatUint(uint64(uid), 10) //先类型转换

		// best
		sc, err := global.RedisDB.ZScore(zsetKey, member).Result()
		if err != nil && err != redis.Nil { //这里排除真正的错误
			return 0, 0, err
		}

		if err == nil { // 在 Top10 里
			best = int(sc)
			// rank（0-based -> 1-based）
			var r0 int64
			var e2 error

			if isLowerBetter {
				// 分数越低排名越靠前
				r0, e2 = global.RedisDB.ZRank(zsetKey, member).Result()
			} else {
				// 分数越高排名越靠前
				r0, e2 = global.RedisDB.ZRevRank(zsetKey, member).Result()
			}

			if e2 != nil && e2 != redis.Nil {
				return best, 0, e2
			}
			if e2 == nil {
				return best, int(r0) + 1, nil // 在 Top10 里返回redis的排名数据
			}
			// 理论上走不到这里
		}
	}

	// 不在 Top10即空结果：直接回落到 MySQL 查历史最佳
	best, err = queryBestFromDB(uid, zsetKey) //返回最佳分数
	if err != nil {
		return 0, 0, err
	}

	return best, 0, nil
}

// --------- 接口：GET /api/game/leaderboard/me?game=guess_game ---------
// GameLeaderboardMe 获取排行榜及当前用户在排行榜中的位置
// @Summary 获取游戏排行榜及个人排名
// @Description 返回指定游戏的公共排行榜Top10和当前用户在该游戏中的成绩与排名
// @Description 不传game参数时，返回所有游戏的排行榜和个人数据
// @Description 传game参数时，只返回指定游戏的排行榜和个人数据
// @Tags GameLeaderboard
// @Accept json
// @Produce json
// @Param game query string false "游戏代号（可选值：guess_game, map_game, 2048_game）不传则返回所有游戏"
// @Security ApiKeyAuth
// @Success 200 {object} object{games=object{guess_game=object{leaderboard=[]LBEntry,my_rank=object{best=int,rank=int}}}} "返回所有游戏排行榜（不传game参数）"
// @Success 200 {object} object{game=string,leaderboard=[]LBEntry,my_rank=object{best=int,rank=int,user_id=int,username=string}} "返回单个游戏排行榜（传game参数）"
// @Failure 400 {object} object{error=string} "无效的游戏代号"
// @Failure 401 {object} object{error=string} "未授权"
// @Failure 500 {object} object{error=string} "服务器内部错误"
// @Router /api/game/leaderboard/me [get]
func GameLeaderboardMe(c *gin.Context) {

	select {
	case leaderboardLimiter <- struct{}{}: //送入空的结构体
		defer exit_thread()
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
		return
	}

	uid := c.GetUint("user_id")
	uname := c.GetString("username")
	if uid == 0 || uname == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 顺手把用户名写入统一 Hash（幂等；避免老数据没有名字） - 防御性编程
	if global.RedisDB != nil && uname != "" {
		_ = global.RedisDB.HSet(config.RedisKeyUsernames, strconv.FormatUint(uint64(uid), 10), uname).Err()
	}

	// 支持多种功能的查询
	// 游戏代号：如果指定了game参数，只返回该游戏；否则返回所有游戏
	gameCode := c.Query("game")
	if gameCode != "" { // 如果指定了游戏，返回该游戏的排行榜和个人数据
		zkey, ok := boards[gameCode]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game code, available: guess_game, map_game, 2048_game"})
			return
		}

		isLowerBetter := lowerIsBetter[gameCode]

		// 读取当前公共排行榜 Top10
		leaderboard, err := readTopN(topK, zkey, isLowerBetter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read leaderboard"})
			return
		}

		// 获取当前用户的成绩和排名
		best, rank, err := myBestAndRank(zkey, uid, isLowerBetter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read user rank"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"game":        gameCode,
			"leaderboard": leaderboard, // Top10 排行榜
			"my_rank": gin.H{ // 我的排名信息
				"username": uname,
				"best":     best,
				"rank":     rank, // 0表示未上榜
			},
		})
		return
	}

	// 未指定游戏：返回所有游戏的排行榜和个人数据
	result := make(map[string]gin.H)
	errors := make(map[string]string)

	for code, zkey := range boards { // 遍历3个游戏
		isLowerBetter := lowerIsBetter[code]

		// 获取公共排行榜
		leaderboard, err := readTopN(topK, zkey, isLowerBetter)
		if err != nil {
			errors[code] = "failed to read leaderboard: " + err.Error()
			continue
		}

		// 获取个人数据
		best, rank, err := myBestAndRank(zkey, uid, isLowerBetter)
		if err != nil {
			errors[code] = "failed to read user rank: " + err.Error()
			continue
		}

		result[code] = gin.H{
			"leaderboard": leaderboard, // Top10 排行榜
			"my_rank": gin.H{ // 我的排名信息
				"best": best,
				"rank": rank,
			},
		}
	}
	resp := gin.H{
		"username": uname,
		"games":    result,
	}
	if len(errors) > 0 {
		resp["errors"] = errors
	}

	c.JSON(http.StatusOK, resp)
}

// 这里是从Redis数据库中查询某个用户的最佳成绩
// 根据 zsetKey 判断是哪个游戏，查询对应的表
func queryBestFromDB(uid uint, zsetKey string) (int, error) { //获取当前用户的最高分数
	if uid == 0 {
		return 0, nil
	}
	var best int //获取每个用户的分数
	var err error

	// 根据 zsetKey 判断是哪个游戏-全部计算
	switch zsetKey {
	case config.RedisKeyTop10Best:
		// 猜数字游戏：查询最高分
		err = global.DB.
			Model(&models.Game_Guess_Score{}).
			Where("user_id = ?", uid).
			Select("MAX(score) AS best").
			Scan(&best).Error
	case config.RedisKeyTop10FastestMap:
		// 地图游戏：查询最短时间（score字段存储浮点数时间）
		var timeScore float64
		err = global.DB.
			Model(&models.Game_Map_Time{}).
			Where("user_id = ?", uid).
			Select("MIN(score) AS best").
			Scan(&timeScore).Error
		if err == nil {
			best = int(timeScore * 100) // 转换为整数方便返回（保留2位小数精度）
		}
	case config.RedisKeyTop10Game2048:
		// 2048游戏：查询最高分
		err = global.DB.
			Model(&models.Game_2048_Score{}).
			Where("user_id = ?", uid).
			Select("MAX(score) AS best").
			Scan(&best).Error
	default:
		// 未知游戏类型
		return 0, nil
	}

	if err != nil {
		return 0, err
	}
	return best, nil
}
