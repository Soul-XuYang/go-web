// models/game_score.go
package models

import (
	"time"

	"gorm.io/gorm"
)

// 一局游戏的最终得分-外键连接
type Game_Guess_Score struct {
	ID        uint           `gorm:"primaryKey"`     // 不用
	UserID    uint           `gorm:"index;not null"` //外键-关键
	UserName  string         `gorm:"not null"`
	User      Users          `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Score     int            `gorm:"not null"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Game_Guess_Score) TableName() string { return "game_guess_scores" }

// 一局游戏的最终用时-外键连接（使用Score字段存储时间，便于排行榜通用函数）
type Game_Map_Time struct {
	ID        uint           `gorm:"primaryKey"`     // 不用
	UserID    uint           `gorm:"index;not null"` //外键-关键
	UserName  string         `gorm:"not null"`
	User      Users          `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Score     float64        `gorm:"not null"` // 存储时间（秒），便于排行榜通用函数使用
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Game_Map_Time) TableName() string { return "game_map_times" }
