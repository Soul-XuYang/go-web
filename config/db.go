package config

import (
	"fmt"
	"log"
	"project/global"
	"project/models"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func initDB() { //注意这个是小写只能在当前包使用，大写才能被其他包使用
	dsn := AppConfig.Database.Dsn
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{}) // 连接数据库 open ，gorm.Config是配置项
	if err != nil {
		log.Fatalf("DataBase connection failed ,got error:%v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to set connection pool ,got error:%v", err)
	}
	sqlDB.SetMaxIdleConns(AppConfig.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(AppConfig.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(AppConfig.Database.ConnMaxLifetimeHours) * time.Hour) // 设置最大连接时间,连接1h后就断开了连接
	global.DB = db
	fmt.Println("DataBase connection success!")
}
func runMigrations() {
	if err := global.DB.AutoMigrate(
		&models.Users{},
		&models.Article{},
		&models.ExchangeRate{},
	); err != nil {
		log.Fatalf("auto migrate error: %v", err)
	}
}
