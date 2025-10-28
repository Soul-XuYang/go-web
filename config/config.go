package config // 建立包

import (
	"errors"
	"fmt"
	"log"
	"project/global"
	"project/models"
	"project/utils"
	"strings"

	"github.com/spf13/viper"
	"gorm.io/gorm"
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
	Superadmin struct {
		Username string
		Password string
	}
	Api struct {
		LocalKey           string
		LocationDailyLimit int
	}
}

var AppConfig *Config //创建配置文件-指针全局可以修改并且避免拷贝-配置句柄
var LocalAPIKey string

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
	LocalAPIKey = AppConfig.Api.LocalKey
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
	fmt.Printf("Login URL:http://localhost%s/auth/login\n", GetPort())
}

func superadmin_init() {
	if AppConfig == nil || AppConfig.Superadmin.Username == "" || AppConfig.Superadmin.Password == "" {
		log.Println("Warning: Superadmin credentials are not set in config file")
		return
	}
	username := AppConfig.Superadmin.Username
	rawPass := AppConfig.Superadmin.Password

	// 先计算哈希（不要先写明文再更新）
	hashed, err := utils.HashPassword(rawPass)
	if err != nil {
		log.Fatalf("hash superadmin password failed: %v", err)
	}

	var u models.Users

	// 1) Unscoped 查询，能够看到软删除数据-这里是先看软数据
	tx := global.DB.Unscoped().Where("username = ?", username).First(&u) //这里查询用户
	switch {
	case errors.Is(tx.Error, gorm.ErrRecordNotFound):
		// 2) 确实不存在 → 直接用哈希创建
		u = models.Users{
			Username: username,
			Password: hashed, // 存哈希
			Role:     "superadmin",
		}
		if err := global.DB.Create(&u).Error; err != nil {
			log.Fatalf("create superadmin failed: %v", err)
		}
	case tx.Error != nil:
		log.Fatalf("query superadmin failed: %v", tx.Error)
	default:
		// 3) 已存在：若软删除则恢复
		if u.DeletedAt.Valid {
			if err := global.DB.Unscoped().Model(&u).Update("deleted_at", nil).Error; err != nil {
				log.Fatalf("undelete superadmin failed: %v", err)
			}
		}
		//  校正角色为 superadmin；若不是 bcrypt（粗判 $2 开头），则更新哈希-保险操作
		updates := map[string]any{}
		if !strings.EqualFold(u.Role, "superadmin") {
			updates["role"] = "superadmin"
		}
		if !strings.HasPrefix(u.Password, "$2") { // 不是 bcrypt 哈希
			updates["password"] = hashed
		}
		if len(updates) > 0 {
			if err := global.DB.Model(&u).Updates(updates).Error; err != nil {
				log.Fatalf("update superadmin failed: %v", err)
			}
		}
	}

	fmt.Println("3. Superadmin has already initializated!")
}

// test -删除superadmin用户
func deleteSuperadminHard() {
	res := global.DB.Unscoped().Where("username = ?", AppConfig.Superadmin.Username).Delete(&models.Users{})
	if res.Error != nil {
		fmt.Printf("hard delete failed: %v\n", res.Error)
		return
	}
	fmt.Printf("The superadmin has been hard deleted %d rows\n", res.RowsAffected)
}
