package models

import "gorm.io/gorm"

type Article struct{
    gorm.Model
    User    *Users    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束与级联
    UserID uint `gorm:"index"`
    Title string `binding:"required"`
    Content string  `binding:"required"`
    Preview string  `binding:"required"`
    Likes int `gorm:"default:0"`
}

func (Article) TableName() string { return "articles" }