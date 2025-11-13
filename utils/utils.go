package utils

// 辅助工具函数
import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	cipher_number = 12 //自动识别类型
	Expire_hours  = 72
	default_role  = "user"
)

func HashPassword(pwd string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), cipher_number)
	return string(hash), err
}
func GenerateJWT(username string, role string) (string, error) {
	// 用 MapClaims 时，直接传入 jwt.MapClaims{...}
	claims := jwt.MapClaims{
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(time.Duration(Expire_hours) * time.Hour).Unix(), // 过期时间（秒）
		"iat":      time.Now().Unix(),                                              // 签发时间（可选）
		"nbf":      time.Now().Unix(),                                              // 生效时间（可选）
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// 生产环境：把 "secret" 放到配置/环境变量里
	signedToken, err := token.SignedString([]byte("secret"))
	return "Bearer " + signedToken, err // 注意 Bearer 后面要有空格
}

// 因为这里我们的token包含了username信息
func ParseJWT(tk string) (string, string, int64, error) {
	tk = strings.TrimSpace(tk) // TrimSpace去除字符串两端的空白字符
	low := strings.ToLower(tk) // 将字符串转换为小写
	if strings.HasPrefix(low, "bearer ") {
		tk = strings.TrimSpace(tk[7:]) //7-前缀长度
	}
	if tk == "" {
		return "", default_role, 0, errors.New("empty token")
	}
	token, err := jwt.Parse(tk, func(token *jwt.Token) (interface{}, error) { // 这里依据其框架写入对应实现的函数操作
		// 固定算法族
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok { //按照这个HMAC法解析
			return nil, jwt.ErrTokenUnverifiable
		}
		return []byte("secret"), nil
	})
	if err != nil {
		return "", default_role, 0, err
	}
	//  用ok和valid看是否解析成功且声明存在
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// 我们这里往JWT中传入的键值是username和role
		username, ok1 := claims["username"].(string) //获得其键值
		role, ok2 := claims["role"].(string)
		// exp 字段在 JSON 解析时会被解析为 float64，需要先断言为 float64 再转换为 int64
		var expireTime int64
		var ok3 bool
		// 这里多层判断
		if expFloat, ok := claims["exp"].(float64); ok {
			expireTime = int64(expFloat)
			ok3 = true
		} else if expInt, ok := claims["exp"].(int64); ok {
			expireTime = expInt
			ok3 = true
		}
		if !ok1 || !ok2 || !ok3 {
			return "", default_role, 0, errors.New("user's claim is not a string")
		}
		return username, role, expireTime, nil
	}
	return "", default_role, 0, errors.New("invalid token")
}
