// models/game_score.go
package models

import (
	"time"

	"gorm.io/gorm"
)

// 一局游戏的最终得分-外键连接
type Game_Guess_Score struct {
	ID        uint           `gorm:"primaryKey"`
	UserID    uint           `gorm:"index;not null"` // 👉 外键列
	User      Users          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID;references:ID"`
    // foreignKey:UserID - 指定了当前表（Game_Guess_Score）中的外键列是 UserID，references:ID 指定了关联表（Users）中的主键列是 ID。
	Score     int            `gorm:"not null"`       // 得分
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Game_Guess_Score) TableName() string { return "game_guess_scores" }
