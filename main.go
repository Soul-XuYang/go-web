package main

import (
	"log"
	"project/config"
	"project/router"

	"github.com/gin-gonic/gin"
)

type Info struct {
	Message string `json:"message"` // 编译时是字符串，运行是认为其是json-反射
}

func main() {
	log.Println("The main app has runnned:")
	config.InitConfig()       // 初始化配置-只对包里的全局变量初始化
	r := router.SetupRouter() // 单独的路由设置
	//单独的方法
	r.GET("/hello", func(c *gin.Context) { //设立请求路径和方法以及对应的函数
		c.JSON(200, Info{Message: "Hello, World!"})
	})
	port := config.GetPort() // 获取端口-这里config是包名
	r.Run(port)              // 监听端口并启动服务
}

//   login的测试数据
//   "username": "wxy",
//   "password": "123456"
