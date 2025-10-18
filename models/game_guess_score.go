// models/game_score.go
package models

import (
	"time"

	"gorm.io/gorm"
)

// 一局游戏的最终得分-外键连接
type Game_Guess_Score struct {
	ID        uint           `gorm:"primaryKey"`  // 不用
	UserID    uint           `gorm:"index;not null"` //外键-关键
	UserName  string         `gorm:"not null"` 
	User      Users          `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Score     int            `gorm:"not null"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}


func (Game_Guess_Score) TableName() string { return "game_guess_scores" }
