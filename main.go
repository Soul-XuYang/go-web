package main

import (
	"os"
	"project/config"
	_ "project/docs" // 👈 swag init 后会生成
	"project/log"
	"project/router"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @title       Go-Web项目 API
// @version     0.0.1
// @description Go-Web 综合性Web应用接口文档
// @BasePath    /api
func main() {
	//初始化日志以及监控代码程序
	if err := log.Init(false); err != nil { // 初始化日志-false 表示开发模式
		panic(err)
	}
	defer log.Sync() //确保日志写入
	Monitor := log.NewMonitor()
	dir, err := os.Getwd()
	if err != nil {
		log.L().Error("Failed to get Path", zap.Error(err))
	}
	Monitor.StartMonitor(dir)
	defer Monitor.StopMonitor()

	//配置初始化
	gin.SetMode(gin.ReleaseMode) // 设置gin的模式
	config.InitConfig()          // 初始化配置-只对包里的全局变量初始化
	r := router.SetupRouter()    // 路由设置
	port := config.GetPort()     // 获取端口-这里config是包名

	//运行程序并监听端口
	log.L().Info("The main app has runnned!")
	r.Run(port) // 监听端口并启动服务
}

//  开发测试的数据
//   login的测试数据
//   "username": "inkkaplum123456",
//   "password": "123456"
