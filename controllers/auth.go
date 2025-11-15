package controllers

// auth 身份认证 -包含各种对应路由的操作函数
import (
	"errors"
	"fmt"
	"net/http"
	"project/config"
	"project/global"
	"project/models"
	"project/utils"
	"time"

	"golang.org/x/time/rate"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// DTO数据
type RegisterDTO struct {
	Username string `json:"username" binding:"required,alphanum,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=64"`
}

type LoginDTO struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register godoc
// @Summary     用户注册
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       body  body      controllers.RegisterDTO  true  "注册参数"
// @Success     200   {object}  map[string]string
// @Failure     400   {object}  map[string]string
// @Router      /auth/register [post]
func Register(c *gin.Context) {
	var in RegisterDTO //注册的DTO
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !registerLimiter(c, in.Username) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts, please try again later"})
		return
	}
	uname := in.Username

	hash, err := utils.HashPassword(in.Password) // 对其加密
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
		return
	}

	u := models.Users{Username: uname, Password: hash} //赋值,默认注册的用户都是普通用户

	if err := global.DB.Create(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "username has already existed"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 建议：写库成功后再签发JWT
	token, err := utils.GenerateJWT(u.Username, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate token failed"})
		return
	}
	utils.SetAuthCookie(c, token, utils.Expire_hours*time.Hour) //给上下文签发token和过i时间
	c.JSON(http.StatusCreated, gin.H{"token": token})
}
func CheckPassword(hash string, pwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) //第一个是hash加密过的密码，第二个是原装的密码-并不是字符串的比较
	return err == nil
}

// Login godoc
// @Summary     用户登录
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       body  body      controllers.LoginDTO  true  "登录参数"
// @Success     200   {object}  map[string]string  "token,result_url"
// @Failure     400   {object}  map[string]string
// @Router      /auth/login [post]   // 注意：不要写 /api，已由 @BasePath /api 补齐
func Login(c *gin.Context) {
	var in LoginDTO
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uname := in.Username
	if in.Username == "" || in.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}
	// 添加请求频率限制（使用 Redis 限流，支持分布式）
	if !loginLimiterRedis(in.Username) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts, please try again later"})
		return
	}

	var user models.Users
	if err := global.DB.Where("username = ?", uname).First(&user).Error; err != nil {
		// 不区分“用户不存在/密码错误”，统一提示，避免枚举用户名
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	if !CheckPassword(user.Password, in.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, err := utils.GenerateJWT(user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate token failed"})
		return
	}
	Result_Url := "/page/shell" // 登录成功后跳转的页面
	if user.Role != "user" {
		Result_Url = "/admin/dashboard"
	}
	utils.SetAuthCookie(c, token, utils.Expire_hours*time.Hour) //设定cookie
	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"result_url": Result_Url,
	})
}

// Logout godoc
// @Summary     退出登录
// @Tags        Auth
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  map[string]string
// @Router      /auth/logout [post]
// controllers/auth.go
func Logout(c *gin.Context) {
	utils.ClearAuthCookie(c)
	c.JSON(200, gin.H{"ok": true})
}

type deleteInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// @Summary      Delete user account
// @Description  Delete the current user's account after password verification
// @Tags         User
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        data  body      deleteInput  true  "User credentials"
// @Success      200   {object}  map[string]interface{}  "退出成功"
// @Failure      400   {object}  map[string]interface{}  "请求参数错误"
// @Failure      401   {object}  map[string]interface{}  "未认证"
// @Failure      500   {object}  map[string]interface{}  "服务器错误"
// @Router       /user/delete [delete]
// 这个是在登录之后的注销页面
func DeleteUser(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var deleteInput deleteInput //获得其请求的数据
	if err := c.ShouldBindJSON(&deleteInput); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var user models.Users
	if err := global.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	if !CheckPassword(user.Password, deleteInput.Password) || user.Username != deleteInput.Username {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}
	if err := global.DB.Delete(&deleteInput).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	utils.ClearAuthCookie(c)
	c.JSON(http.StatusOK, gin.H{
		"ok": true,
	})
}

// 本地限流版本（如果是当前的单实例场景，性能更好）-注意如果使用这个函数main里要开启限流清楚器
func loginLimiterLocal(username string) *rate.Limiter {
	limiter, _ := config.LoginAttempts.LoadOrStore(username, rate.NewLimiter(5, 5)) // 每秒5次，突发5次
	return limiter.(*rate.Limiter)                                                  //类型断言
}

// Redis 限流版本（分布式场景，与项目中其他限流保持一致）
// 使用滑动窗口算法：60秒内最多5次登录尝试
func loginLimiterRedis(username string) bool {
	rateKey := fmt.Sprintf(config.RedisLoginRate, username) //缓存的key-表名
	now := time.Now().Unix()
	window := int64(config.RedisWindow)
	maxAttempts := int64(config.RedisRateMaxAttempts) // 最多5次

	pipe := global.RedisDB.Pipeline()
	// 清理过期记录（60秒前的）
	pipe.ZRemRangeByScore(rateKey, "0", fmt.Sprintf("%d", now-window)) //删除
	// 统计当前窗口内的请求数
	pipe.ZCard(rateKey)
	// 添加当前请求的时间戳
	pipe.ZAdd(rateKey, redis.Z{Score: float64(now), Member: fmt.Sprintf("%d", now)}) //ZSet是排序集合-Score为排序的元素，Member为对应存储的值
	// 设置过期时间
	pipe.Expire(rateKey, time.Duration(window)*time.Second)
	results, err := pipe.Exec() //一次性执行所有命令

	if err != nil {
		return true
	}

	// 获取当前窗口内的请求数
	count := results[1].(*redis.IntCmd).Val()
	return count <= maxAttempts
}
func registerLimiter(c *gin.Context, username string) bool {
	clientIP := c.ClientIP()
	ipKey := fmt.Sprintf(config.RedisRegisterRateIP, clientIP)
	ipCount, err := global.RedisDB.Incr(ipKey).Result() //获得其计数
	if err == nil {
		if ipCount == 1 {
			global.RedisDB.Expire(ipKey, config.RedisRegisterRateTTL) // 第一次设置10分钟过期
		}
		if ipCount > config.RedisRateMaxAttempts {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "注册过于频繁,请" + config.RedisRegisterRateTTL.String() + "后再试",
			})
			return false
		}
	}

	usernameKey := fmt.Sprintf(config.RedisRegisterRateUser, username)
	usernameCount, err := global.RedisDB.Incr(usernameKey).Result()
	if err == nil {
		if usernameCount == 1 {
			global.RedisDB.Expire(usernameKey, config.RedisRegisterRateTTL)
		}
		if usernameCount > config.RedisRateMaxAttempts {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "该用户名注册尝试过于频繁，请于" + config.RedisRegisterRateTTL.String() + "后再试",
			})
			return false
		}
	}
	return true
}
