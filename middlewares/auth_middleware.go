package middlewares

import (
	"net/http"
	"project/utils"

	"github.com/gin-gonic/gin"
)

//自定义中间件
func AuthMiddleWare() gin.HandlerFunc{//返回的是gin下的函数类型
    return func(c *gin.Context){
        token:= c.GetHeader("Authorization") // 这里的键是Authorization
        if token== ""{
            c.JSON(http.StatusUnauthorized,gin.H{"error": "Unauthorized"})
            c.Abort() //不中止
            return
        }
        username,err:=utils.ParseJWT(token)
        if err!=nil{
            c.JSON(http.StatusUnauthorized,gin.H{"error": err.Error()})
            c.Abort()
            return
        }
        c.Set("username",username)  
        c.Next() //调用下列的函数
    }
}