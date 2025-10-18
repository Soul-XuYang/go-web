package controllers
import (
    "strconv"
    "project/global"
    
    "net/http"
    "github.com/gin-gonic/gin"
    "project/log"
    "go.uber.org/zap"
)
//数据的输入格式
type LBEntry struct {
    UserID   uint   `json:"user_id"`
    Username string `json:"username"`
    Score    int    `json:"score"`
    Rank     int    `json:"rank"` // 1-based
}

func GameLeaderboards(c *gin.Context) {
	result := make(map[string][]LBEntry, len(boards))  // 前面是切片,后面是长度1
	errors := make(map[string]string) // 可选：某个榜读取失败也不影响其他榜

	for gameCode, zkey := range boards {  //这里只有一个面板
		items, err := readTopNBest(topK,zkey)  // items返回的是LBEntry的切片数据
		if err != nil {
			errors[gameCode] = err.Error() // 对应游戏的名称错误
			continue
		}
		result[gameCode] = items
	}
    // 返回的数据
	resp := gin.H{
		"leaderboards": result,   // 这里是返回的格式
	}
	if len(errors) > 0 {
		resp["errors"] = errors // 某些榜失败时返回错误信息（可视需求移除）-新增错误信息
        log.L().Warn("Some game leaderboards read failed", zap.Any("errors", errors)) //打印出来
	}
	c.JSON(http.StatusOK, resp)
}

// 这里默认取10条-输入数据-可以针对任何表格
func readTopNBest(limit int,zest_collection string) ([]LBEntry, error) {
    if global.RedisDB == nil {
        return nil, nil
    }
    if limit <= 0 || limit > 50 {
        limit = 10
    }

    // 高分在前取 0..limit-1
    zs, err := global.RedisDB.ZRevRangeWithScores(zest_collection, 0, int64(limit-1)).Result()  // 给出集合key的名称
    if err != nil {
        return nil, err
    }
    out := make([]LBEntry, 0, len(zs))  // 构建数据的输入格式
    if len(zs) == 0 {
        return out, nil
    }

    // for循环批量取用户名-依据id取名字
    u_id := make([]string, 0, len(zs))
    for _, z := range zs {
        u_id = append(u_id, z.Member.(string))
    }
    names, _ := global.RedisDB.HMGet(redis_name_collection, u_id...).Result()

    for i, z := range zs {
        idStr := z.Member.(string)
        uid64, _ := strconv.ParseUint(idStr, 10, 64) // 转换为uint类型
        name := idStr
        if i < len(names) && names[i] != nil {
            if s, ok := names[i].(string); ok && s != "" {
                name = s
            }
        }
        out = append(out, LBEntry{
            UserID:   uint(uid64),
            Username: name,
            Score:    int(z.Score),
            Rank:     i + 1,
        })
    }
    return out, nil
}

