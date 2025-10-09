package models

import "time"

type ExchangeRate struct { // 汇率信息-并设立精度
    ID           uint64         `gorm:"column:_id;primaryKey;autoIncrement" json:"_id"` // 明确列名为 _id，且主键、自增；64位就用 uint64
    FromCurrency string         `gorm:"size:10;index:idx_pair_date,priority:1" json:"fromCurrency" binding:"required"`
    ToCurrency   string         `gorm:"size:10;index:idx_pair_date,priority:2" json:"toCurrency"   binding:"required"`
    Rate         float64        `gorm:"type:decimal(18,8)" json:"rate" binding:"required"`
    Date         time.Time      `json:"date"` // JSON 默认用 RFC3339，如 "2025-10-09T00:00:00Z"
}

func (ExchangeRate) TableName() string { return "exchange_rates" } // 显式表名
