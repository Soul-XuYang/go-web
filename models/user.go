package models

import "gorm.io/gorm"

// 用户数据
type Users struct {
	gorm.Model        //内嵌的一个模型 包括基础的ID 创建、更新、删除的时间戳
	Username   string `gorm:"unique"`
	Password   string
}

// 显示使用名称
func (Users) TableName() string {
	return "users"
}
