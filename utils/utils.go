package utils

// 辅助工具函数
import (
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
    "time"
)

const cipher_number  = 12  //自动识别类型
const expire_hours = 72
func HashPassword(pwd string) (string,error){
    hash,err := bcrypt.GenerateFromPassword([]byte(pwd),cipher_number)
    return string(hash),err
}
func GenerateJWT(username string) (string, error) {
	// 用 MapClaims 时，直接传入 jwt.MapClaims{...}
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Duration(expire_hours) * time.Hour).Unix(), // 过期时间（秒）
		"iat":      time.Now().Unix(),                                             // 签发时间（可选）
		"nbf":      time.Now().Unix(),                                             // 生效时间（可选）
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// 生产环境：把 "secret" 放到配置/环境变量里
	signedToken, err := token.SignedString([]byte("secret"))
	return "Bearer " + signedToken, err // 注意 Bearer 后面要有空格
}