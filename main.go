package main

import (
	"exchangeapp/config"
	"exchangeapp/router"
	"fmt"
	"github.com/gin-gonic/gin"
    "gorm.io/gorm"
    "gorm.io/driver/mysql"
)
type Info struct {
    Message string `json:"message"`  // 编译时是字符串，运行是认为其是json
}

func main() {
    config.InitConfig()  // 初始化配置-只对包里的全局变量初始化
    fmt.Printf("Port%s\n", config.AppConfig.App.Port)
    r := router.SetupRouter()  // 路由设置
    r.GET("/hello", func(c *gin.Context) {  //设立请求路径和方法以及对应的函数
    c.JSON(200, Info{Message: "Hello, World!"})
    })
    port:=config.GetPort()  // 获取端口-这里config是包名
    r.Run(port) // 监听端口并启动服务
}