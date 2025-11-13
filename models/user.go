package models

import "gorm.io/gorm"


// 用户角色常量
const (
    RoleNormal   = "user"   // 普通用户
    RoleAdmin    = "admin"    // 管理员
    RoleSuperAdmin = "superadmin"  // 超级管理员
)

// 用户数据
type Users struct {
	gorm.Model        //内嵌的一个模型 包括基础的ID 创建、更新、删除的时间戳
	Username   string `gorm:"size:64;uniqueIndex"`
	Password   string
	Role string  `gorm:"type:varchar(16);not null;default:'user';check:role in ('user','admin','superadmin')"` // 用户角色
	Status string  	// 用户状态-可随意填写
}

// 显示使用名称
func (Users) TableName() string {
	return "users"
}
