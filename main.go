package main

import (
	"project/config"
	"project/log"
	"project/router"

	_ "project/docs" // 👈 swag init 后会生成

	"github.com/gin-gonic/gin"
)

type Info struct {
	Message string `json:"message"` // 编译时是字符串，运行是认为其是json-反射
}

// @title       Go_project API
// @version     0.0.1
// @description 接口文档
// @BasePath    /api
func main() {
	// 初始化日志
	if err := log.Init(false); err != nil { // false 表示开发模式
		panic(err)
	}
	defer log.Sync()
	log.L().Info("The main app has runnned!")
	//配置初始化
	config.InitConfig()       // 初始化配置-只对包里的全局变量初始化
	r := router.SetupRouter() // 单独的路由设置
	//单独的方法
	r.GET("/hello", func(c *gin.Context) { //设立请求路径和方法以及对应的函数
		c.JSON(200, Info{Message: "Hello, World!"})
	})
	port := config.GetPort() // 获取端口-这里config是包名
	r.Run(port)              // 监听端口并启动服务
}

//  开发测试的数据
//   login的测试数据
//   "username": "inkkaplum123456",
//   "password": "123456"
