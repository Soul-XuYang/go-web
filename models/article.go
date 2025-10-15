package models

import "gorm.io/gorm"

const My_blog_url = "https://soul-xuyang.github.io/Web_test.github.io/"

type Article struct {
	gorm.Model
	User     *Users `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束与级联
	UserID   uint   `gorm:"index"`                                         // 外键关系
	Title    string `binding:"required"`
	Content  string `gorm:"type:longtext"`  //长文本
	Preview  string `binding:"required"`
	Likes    int    `gorm:"default:0"`
}

func (Article) TableName() string { return "articles" }
