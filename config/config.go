package config // 建立包

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct { //标明这个配置文件是可以全局使用的
	App struct {
		Name string
		Port string
	}
	Database struct {
		Dsn                  string
		MaxIdleConns         int
		MaxOpenConns         int
		ConnMaxLifetimeHours int
	}
	Redis struct {
		Addr     string
		DB       int
		Password string
	}
}

var AppConfig *Config //创建配置文件-指针全局可以修改并且避免拷贝-配置句柄

// 使用viper读取配置文件
func InitConfig() {
	viper.SetConfigName("config") //无extension
	viper.SetConfigType("yml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err) // %v是错误信息的占位格式
		// Fatal 和 Fatalf是可以读取
	}

	AppConfig = &Config{}                              // 创建结构体
	if err := viper.Unmarshal(AppConfig); err != nil { //将配置文件中的内容解析到结构体中
		log.Fatalf("Error unmarshalling config file: %v", err)
	}
	initDB()
	initRedis()
	runMigrations()
	printURL()
}
func GetPort() string {
	if AppConfig == nil || AppConfig.App.Port == "" { //要么配置为空要么端口无
		log.Println("Warning: Port is not set in config file, using default port 8080")
		return ":8080"
	}
	// 确保端口格式正确
	port := AppConfig.App.Port
	if port[0] != ':' {
		port = ":" + port
	}
	return port
}
func printURL() {
	fmt.Printf("Login:http://localhost%s/auth/login\n", GetPort())
}
