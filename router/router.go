package router

//路由组-分组
import (
	"project/controllers"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()           //默认创建
	auth := r.Group("/api/auth") //给出路由组的路径
	auth.POST("/login", controllers.Login)
	auth.POST("/register", controllers.Register)
	return r //返回路由组
}
