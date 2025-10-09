package controllers

// auth 身份认证 -包含各种对应路由的操作函数
import (
	"fmt"
	"net/http"
	"project/global"
	"project/models"
	"project/utils"

	"github.com/gin-gonic/gin"
)

func Register(c * gin.Context){  //对应的注册函数
    var user models.Users // 创建一个变量
    if err :=c.ShouldBindJSON(&user);err!=nil{   // 请求体是Body，对应的数据传入user中
        c.JSON(http.StatusBadRequest,gin.H{"Error Request!": err.Error()}) // 客户端的异常请求
        return 
    }
    hashedPwd,err := utils.HashPassword(user.Password)
    if err!=nil{
        c.JSON(http.StatusBadRequest,gin.H{"Error hashedpassword!": err.Error()})
    }
    user.Password = hashedPwd
    // 注意这里只有密码完成之后才可以进行JWT操作
    token, err := utils.GenerateJWT(user.Username)
    if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"Error JWT!": err.Error()})
    return
    }
    if err:=global.DB.AutoMigrate(&user);err!=nil{
        c.JSON(http.StatusInternalServerError,gin.H{"Error DB AutoMigrate!": err.Error()})
        return
        
    }
    if err := global.DB.Create(&user).Error; err != nil {
      c.JSON(http.StatusInternalServerError, gin.H{"Error DB Created!": err.Error()})
     return
    }
    c.JSON(200,gin.H{"token":token})  //返回token数据-标明创建成功
    fmt.Println(user.Username+"has created succseeful !")
}
func Login(c * gin.Context){
    var input struct{
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err:=c.ShouldBindJSON(&input);err!=nil{

    }

}