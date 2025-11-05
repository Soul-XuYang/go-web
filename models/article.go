package models

import (
	"time"

	"gorm.io/gorm"
)

const My_blog_url = "https://soul-xuyang.github.io/Web_test.github.io/"

// 包括文章的所有元素
type Article struct {
	gorm.Model
	User            *Users    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束与级联
	UserID          uint      `gorm:"index"`                                         // 外键关系
	Title           string    `binding:"required"`
	Content         string    `gorm:"type:longtext"` //长文本
	Preview         string    `binding:"required"`
	Likes           uint      `gorm:"default:0"`
	Comments        []Comment `gorm:"foreignKey:ArticleID"` //这个模型以ArticleID作为外键
	CommentCount    uint      `gorm:"column:comment_count;default:0"`
	// CollectionCount uint      `gorm:"column:collection_count;default:0"` //收藏次数
	RepostCount     uint      `gorm:"default:0"`                         // 由“仍有转发的独立用户数”维护
}
type Comment struct {
	gorm.Model
	Content   string    `gorm:"type:text;not null"`
	User      *Users    `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	UserID    uint      `gorm:"index"`
	ArticleID uint      `gorm:"index"` // 给 ArticleID 加索引也
	Likes     int       `gorm:"default:0"`
	ParentID  *uint     `gorm:"index"`               //这里可以深究为啥用指针-父评论
	Children  []Comment `gorm:"foreignKey:ParentID"` // 子评论
}

// 一个用户可以创建多个收藏夹，一个收藏夹有多篇文章Item
type Collection struct { //
	gorm.Model
	UserID uint   `gorm:"index;not null"`
	User   *Users `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Name   string `gorm:"size:100;not null"`
	// Items     []CollectionItem `gorm:"foreignKey:CollectionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ItemCount uint `gorm:"default:0"` // 冗余统计，便于快速展示
}

// 下方为约束表
type UserLikeArticle struct { //关联表
	UserID    uint      `gorm:"primaryKey"`
	ArticleID uint      `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
type CollectionItem struct {
	gorm.Model
	CollectionID uint
	ArticleID    uint
	Article      *Article    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 单篇文章的所有信息-外键约束
	Collection   *Collection `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束-Collection
}
type UserArticleRepost struct {
	UserID    uint      `gorm:"primaryKey"`
	ArticleID uint      `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (Article) TableName() string           { return "articles" }
func (Comment) TableName() string           { return "comments" }
func (Collection) TableName() string        { return "collections" }
func (CollectionItem) TableName() string    { return "collection_items" }
func (UserLikeArticle) TableName() string   { return "UserLikeArticles" }
func (UserArticleRepost) TableName() string { return "UserArticleReposts" }
