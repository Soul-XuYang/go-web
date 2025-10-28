package controllers

// auth 身份认证 -包含各种对应路由的操作函数
import (
	"errors"
	"net/http"
	"project/global"
	"project/models"
	"project/utils"
	"time"

	"github.com/gin-gonic/gin"
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
// @Param       body  body      controllers.LoginDTO  true  "注册参数"
// @Success     200   {object}  map[string]string
// @Failure     400   {object}  map[string]string
// @Router      /auth/register [post]
func Register(c *gin.Context) {
	var in RegisterDTO //注册的DTO
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	utils.SetAuthCookie(c, token, utils.Expire_hours*time.Hour) //给上下文签发token和
	c.JSON(http.StatusCreated, gin.H{"token": token})
}

func CheckPassword(hash string, pwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) //第一个是hash加密过的密码，第二个是原装的密码
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
