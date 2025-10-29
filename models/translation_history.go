package models

import (
	"gorm.io/gorm"
)

// TranslationHistory 翻译历史记录模型
type TranslationHistory struct {
	gorm.Model  // 历史的基本参数
	User     *Users `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // 外键约束与级联
	UserID        uint      `json:"user_id" gorm:"not null;index"` // 用户ID
	SourceText    string    `json:"source_text" gorm:"type:text;not null"` // 原文
	TranslatedText string    `json:"translated_text" gorm:"type:text;not null"` // 译文
	SourceLang    string    `json:"source_lang" gorm:"size:10;not null"` // 源语言
	TargetLang    string    `json:"target_lang" gorm:"size:10;not null"` // 目标语言
	LLM        string    `json:"model" gorm:"size:50"` // 使用的模型
	Provider      string    `json:"provider" gorm:"size:50"` // API提供商
}

// TableName 指定表名
func (TranslationHistory) TableName() string {
	return "translation_histories"
}
