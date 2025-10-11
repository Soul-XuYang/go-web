// models/rmb_top10.go
package models

import "time"

// RMB 对 Top10 主要货币的“快照表”
type RmbTop10S struct {
	ID     uint      `gorm:"primaryKey"`
	Symbol string    `gorm:"size:3;index"` // 目标币种，如 USD/EUR/...
	Rate   float64   `gorm:"type:decimal(18,6);not null"`// 1 CNY = ? SYMBOL
	Invert float64   `gorm:"type:decimal(18,6);not null"`// 1 SYMBOL = ? CNY（新增） 
	AsOf   time.Time `gorm:"index"` // API 返回的日期（UTC）
}

func (RmbTop10S) TableName() string { return "rmb_top10s" }
