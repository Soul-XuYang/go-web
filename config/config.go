package config // 建立包

import (
	"fmt"
	"log"
	"project/global"
	"project/models"
	"project/utils"

	"github.com/spf13/viper"
)

// 项目版本信息在logger里
const Version string = "0.0.1"

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
	superadmin_init()
	printURL()
}
func GetPort() string {
	if AppConfig == nil || AppConfig.App.Port == "" { //要么配置为空要么端口无
		log.Println("Warning: Port is not set in config file, using default port 8080") //默认端口
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

const (
	superadmin          = "superadmin"
	superadmin_password = "123456"
)

func superadmin_init() {
	u := models.Users{Username: superadmin, Password: superadmin, Role: "superadmin"}
	// FirstOrCreate 会先查找，如果不存在就创建
	if err := global.DB.Where("username = ?", superadmin). //FirstOrCreate
								FirstOrCreate(&u).Error; err != nil {
		fmt.Println("Failed to create/find superadmin:", err)
		return
	}
	// 如果是新创建的记录，需要加密密码
	if u.Password == superadmin { // 密码还是原始值，说明是新创建的
		hashedPassword, err := utils.HashPassword(superadmin_password)
		if err != nil {
			log.Fatal(err)
		}
		if err := global.DB.Model(&u).Update("password", hashedPassword).Error; err != nil {
			fmt.Println("Failed to update password:", err)
			return
		}
	}
	fmt.Println("3. Superadmin has already initializated!")
}

// test -删除superadmin用户
func deleteSuperadminHard() {
	res := global.DB.Unscoped().Where("username = ?", superadmin).Delete(&models.Users{})
	if res.Error != nil {
		fmt.Printf("hard delete failed: %v\n", res.Error)
		return
	}
	fmt.Printf("The superadmin has been hard deleted %d rows\n", res.RowsAffected)
}
