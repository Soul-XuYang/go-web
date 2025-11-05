package models

import (
	"time"

	"gorm.io/gorm"
)

const My_blog_url = "https://soul-xuyang.github.io/Web_test.github.io/"

// 包括文章的所有元素
type Article struct {
	gorm.Model
	User         *Users    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束与级联
	UserID       uint      `gorm:"index"`                                         // 外键关系
	Title        string    `binding:"required"`
	Content      string    `gorm:"type:longtext"` //长文本
	Preview      string    `binding:"required"`
	Likes        uint       `gorm:"default:0"`
	Commentcount uint      `gorm:"column:comment_count;default:0"`
	Comments     []Comment `gorm:"foreignKey:ArticleID"` //这个模型以ArticleID作为外键
}

type Comment struct {
	gorm.Model        // 包含 ID, CreatedAt, UpdatedAt, DeletedAt
	Content    string `gorm:"type:text;not null"`
	User       *Users `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	UserID     uint   `gorm:"index"`
	ArticleID  uint   `gorm:"index"` // 给 ArticleID 加索引也
	Likes      int    `gorm:"default:0"`
	ParentID   *uint  `gorm:"index"`//这里可以深究为啥用指针-父评论
	Children []Comment  `gorm:"foreignKey:ParentID"` // 子评论
}
type UserLikeArticle struct { //关联表
	UserID    uint      `gorm:"primaryKey"`
	ArticleID uint      `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (Article) TableName() string         { return "articles" }
func (Comment) TableName() string         { return "comments" }
func (UserLikeArticle) TableName() string { return "UserLikeArticles" }
