package config

import (
	"fmt"
	"project/global"
	"project/log"
	"time"

	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

// 设置redis表的key
const (
	// redis和lru公共的角色管理表名
	RedisKeyUsers = "user:info:%s"
	// redis的用户表名
	RedisKeyGameUsernames = "game:usernames" //统一的游戏用户表
	//这里是redis中各个游戏的表名
	RedisKeyTop10Best       = "game:guess:top10:best"  // 猜数字的游戏排行榜
	RedisKeyTop10FastestMap = "game:map:top10:fastest" // 地图游戏排行榜（用时最短）
	RedisKeyTop10Game2048   = "game:2048:top10:best"   // 用best表示分数好
	Cache_RateKey           = "rmb_top10:cny"
	//文章缓存
	RedisHomePage = "articles:list:homepage:default" //主页缓存
	// 交互式的缓存 - 读取文章
	RedisLikeKey       = "articles:%d:likes"          //该文章的点赞数
	RedisUserLikeKey   = "articles:%d:user:%d:like"   //关联性点赞
	RedisArticleKey    = "articles:%d"                //判断文章是否存在-bool
	RedisRepostKey     = "articles:%d:reposts"        //该文章的转发数
	RedisUserRepostKey = "articles:%d:user:%d:repost" //关联性转发
	// 时限
	RedisCommentRate          = "comment:rate:user:%d"
	RedisRepostRate           = "repost:rate:user:%d"
	RedisCreateCollectionRate = "collection:rate:user:%d"
	// 注册登录限流
	RedisLoginRate            = "login:rate:%s" // 登录限流 key
	RedisRegisterRateIP       = "register:rate:ip:%s" // IP注册限流 key
	RedisRegisterRateUser     = "register:rate:user:%s" // 用户名注册限流 key
)
const (
	CacheTTL      = 120 * time.Minute // 基本的缓存时间
	LockTTL       = 10 * time.Second  // api请求的上锁时间
	WaitWarmup    = 5 * time.Second
	PollInterval  = 120 * time.Millisecond
	Datasaved_TTL = 12 * time.Hour
	Article_TTL   = 24 * time.Hour
	RoleCacheTTL  = 3 * 24 * time.Hour
    //登录-注册时间限流

)

func initRedis() {
	RedisClient := redis.NewClient(&redis.Options{ //配置选项Options是结构体
		Addr:         AppConfig.Redis.Addr,
		DB:           AppConfig.Redis.DB,
		Password:     AppConfig.Redis.Password,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  800 * time.Millisecond, // 读超时
		WriteTimeout: 800 * time.Millisecond, // 写超时
		PoolSize:     20,
		MinIdleConns: 5,
	}) //返回一个客户端
	_, err := RedisClient.Ping().Result()
	if err != nil {
		log.L().Error("DataBase connection failed ,got error:", zap.Error(err))

	}
	global.RedisDB = RedisClient
	fmt.Println("2. Redis DataBase connection success!")
}
