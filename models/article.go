package models

import "gorm.io/gorm"

type Article struct{
    gorm.Model
    User    *Users    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束与级联
    UserID uint `gorm:"index"`  // 外键关系
    Title string `binding:"required"`
    Content string  `binding:"required"`
    Preview string  `binding:"required"`
    Likes int `gorm:"default:0"`
    Comments string 
}

func (Article) TableName() string { return "articles" }