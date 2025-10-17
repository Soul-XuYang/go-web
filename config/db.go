package config

import (
	"fmt"
	"project/global"
	"project/log"
	"project/models"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func initDB() { //注意这个是小写只能在当前包使用，大写才能被其他包使用
	dsn := AppConfig.Database.Dsn
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{}) // 连接数据库 open ，gorm.Config是配置项
	if err != nil {
		log.L().Fatal("DataBase connection failed",
			zap.Error(err),
			zap.String("dsn", dsn),
		)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.L().Error("DataBase connection failed ,got error:", zap.Error(err))
	}
	sqlDB.SetMaxIdleConns(AppConfig.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(AppConfig.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(AppConfig.Database.ConnMaxLifetimeHours) * time.Hour) // 设置最大连接时间,连接1h后就断开了连接
	global.DB = db
	fmt.Println("1. DataBase connection success!")
}
func runMigrations() {
	if err := global.DB.AutoMigrate(
		&models.Users{},
		&models.Article{},
		&models.ExchangeRate{},
		&models.RmbTop10S{}, // ← 新增,
	    &models.Game_Guess_Score{}, // 新增表
	); err != nil {
		log.L().Error("DataBase connection failed ,got error:", zap.Error(err))
	}
}
