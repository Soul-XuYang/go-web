package router
//路由组-分组
import "github.com/gin-gonic/gin"
func SetupRouter() *gin.Engine { 
    r := gin.Default() //默认创建
    auth:=r.Group("/api/auth") //给出路由组的路径
    auth.POST("/login",func (c *gin.Context){
        c.AbortWithStatusJSON(200,gin.H{
         "msg":"login success!",   
        })
    })

    auth.POST("/register",func (c * gin.Context){
        c.AbortWithStatusJSON(200,gin.H{  //以json文件返回终止信息
         "msg":"login success!",   
        })
    })
    return r //返回路由组
    }