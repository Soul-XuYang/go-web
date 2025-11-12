package controllers

import (
	"project/config"
)

// 这里的id都是user_id而不是表的主键id即guess_score的id

var redis_name_collection = config.RedisKeyGameUsernames

const topK = 10

var boards = map[string]string{
	"guess_game": config.RedisKeyTop10Best,        // 数字猜猜乐（最佳单局 Top10）
	"map_game":   config.RedisKeyTop10FastestMap,  // 地图游戏（用时最短 Top10）
	"2048_game":  config.RedisKeyTop10Game2048,    //2048游戏
	// 后续有新游戏，按同样格式添加：
	// "snake": "game:snake:top10:best",
	
}

// 哈希表标记
var lowerIsBetter = map[string]bool{ 
	"map_game": true, // 地图游戏用时越短越好
}
