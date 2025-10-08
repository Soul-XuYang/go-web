package global

// 供后端代码的全局变量使用
import "gorm.io/gorm"

var (
	DB *gorm.DB // 数据库连接
)
