// utils/jwtcookie.go
package utils

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const CookieName = "Authorization"

func SetAuthCookie(c *gin.Context, token string, ttl time.Duration) {
	// 先设置 SameSite 策略（对后续 SetCookie 生效）
	c.SetSameSite(http.SameSiteLaxMode) // 防大多数 CSRF，站内导航会带上

	// dev 下 secure=false；生产 https 请设为 true
	c.SetCookie(CookieName, token, int(ttl.Seconds()), "/", "", false, true) // HttpOnly
}

func ClearAuthCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, "", -1, "/", "", false, true)
}
