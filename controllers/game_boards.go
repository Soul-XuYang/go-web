package controllers

import (
	"project/config"
)

// 这里的id都是user_id而不是表的主键id即guess_score的id

var redis_name_collection = config.RedisKeyUsernames

const topK = 10

var boards = map[string]string{
	"guess_game": config.RedisKeyTop10Best, // 数字猜猜乐（最佳单局 Top10）
	// 后续有新游戏，按同样格式添加：
	// "snake": "game:snake:top10:best",
	// "2048":  "game:2048:top10:best",
}
