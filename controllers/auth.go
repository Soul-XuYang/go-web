package controllers

// auth 身份认证 -包含各种对应路由的操作函数
import (
	"fmt"
	"net/http"
	"project/global"
	"project/models"
	"project/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func Register(c *gin.Context) { //对应的注册函数
	var user models.Users                           // 创建一个变量
	if err := c.ShouldBindJSON(&user); err != nil { // 请求体是Body，对应的数据传入user中
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}) // 客户端的异常请求
		return
	}
	hashedPwd, err := utils.HashPassword(user.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
	user.Password = hashedPwd
	// 注意这里只有密码完成之后才可以进行JWT操作
	token, err := utils.GenerateJWT(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := global.DB.AutoMigrate(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return

	}
	if err := global.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": token}) //返回token数据-标明创建成功
	fmt.Println(user.Username + "has created succseeful !")
}

func CheckPassword(hash string,pwd string,) bool {  
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) //第一个是hash加密过的密码，第二个是原装的密码
	return err == nil
}
func Login(c *gin.Context) {
	var input struct {
		Username string `json:"username"` //这里的标注是设立json的标签
		Password string `json:"password"`
	}
    // ShouldBind 会根据 Content-Type 自动选解析器（JSON/Form…）
	if err := c.ShouldBindJSON(&input); err != nil {  //这里是把请求体的JSON数据绑定到input里
		c.JSON(http.StatusBadRequest, gin.H{"error:": err.Error()})
		return
	}

	var user models.Users                // 创建一个用户对象
	// 传入输入的用户 First是指按主键升序取第一个记录 -SELECT * FROM users WHERE username = ? ORDER BY id ASC LIMIT 1;GORM框架操作
	if err:=global.DB.Where("username = ?", input.Username).First(&user).Error;err!=nil{
       c.JSON(http.StatusUnauthorized,"This user does not exist!")
	} 
	
	if !CheckPassword( user.Password,input.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error:": "Wrong Password!"})
		return
	}
	token, _ := utils.GenerateJWT(user.Username) // 生成对应JWT的Token返回给客户端
	c.JSON(200, gin.H{"token": token})           
    fmt.Println(input.Username+" has logined successful!")
}
// 注意json大小写不关注
