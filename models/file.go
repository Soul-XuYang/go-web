package models

import (
	"gorm.io/gorm"
)

type Files struct {
	gorm.Model        // ID 为主键
	User       *Users `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束与级联
	UserID     uint   `json:"user_id" gorm:"not null;index"`                 // 上传用户ID
	Filename   string `gorm:"not null;size:255"`                             // 原始文件名
	FileType   string `gorm:"not null;size:50"`                              // MIME 类型或扩展名
	FilePath   string `gorm:"not null;size:500"`                             // 存储路径（本地路径 or URL）
	FileSize   int64  `gorm:"not null"`
	Downloads  uint   `gorm:"default:0"` // 下载数-配合redis缓存
	Hash       string `gorm:"size:64;uniqueIndex"`
	FileInfo   string
	// 这里上传时间就是UpdatedAt
}

func (Files) TableName() string {
	return "files"
}
