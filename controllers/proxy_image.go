package controllers

import (
	"io"
	"net/http"
	"net/url"
	"project/log"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 推荐白名单：把你需要代理的域名放到这里（避免 SSRF）
var proxyAllowedHosts = map[string]bool{
	"mat1.gtimg.com": true,
	// 因为获得的天气的域名如下https://mat1.gtimg.com/qqcdn/xw/tianqi/bigIcon/heiye/02.png
	// "cdn.example.com": true,
}

// ProxyImage 代理前端拉取并透传图片
// @Summary 代理获取图片（透传 Content-Type，缓存 1 小时）
// @Description 从白名单中的图片源以浏览器头伪装拉取资源，透传 Content-Type，并设置 Cache-Control: public, max-age=3600。
// @Tags image, proxy
// @Param url query string true "源图片 URL（需 URL 编码且域名在白名单内）" example(https%3A%2F%2Fmat1.gtimg.com%2Fsome%2Fimage.jpg)
// @Produce octet-stream
// @Success 200 {file} file "图片字节流"
// @Header 200 {string} Content-Type "image/jpeg | image/png | image/webp | image/avif"
// @Header 200 {string} Cache-Control "public, max-age=3600"
// @Failure 400 {object} ErrorResponse "missing url parameter / invalid url / url too long"
// @Failure 403 {object} ErrorResponse "host is not allowed"
// @Failure 502 {object} ErrorResponse "fetch failed / upstream non-200"
// @Failure 500 {object} ErrorResponse "internal"
// @Router /proxy [get]
func ProxyImage(c *gin.Context) {
	raw := c.Query("url") //查询客户端对应的url参数，例如http://yourserver.com/proxy?url=https://mat1.gtimg.com/some/image.jpg中url为后续的网页

	// log.L().Info("ProxyImage called", zap.String("raw", raw), zap.String("client", c.ClientIP()))
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing url parameter"})
		log.L().Warn("ProxyImage:missing url parameter")
		return
	}
	// 查询的链接肯定有问题
	if len(raw) > 4096 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The search url too long"})
		log.L().Warn("ProxyImage:The search url too long")
		return
	}

	u, err := url.Parse(raw) // 解析原始字符串-并以此构建一个URL对象
	if err != nil {
		log.L().Warn("ProxyImage:invalid url", zap.String("raw", raw), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	host := u.Hostname() // 获取url的主机名-目标图片服务器的域名
	if !proxyAllowedHosts[host] {
		log.L().Warn("ProxyImage: The image host not allowed by whitelist", zap.String("host", host))
		c.JSON(http.StatusForbidden, gin.H{"error": "host is not allowed"})
		return
	}
	// 创建一个客户端实例
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 设置请求路由
	req, err := http.NewRequest("GET", raw, nil) //使用这个链接模仿客户端的请求
	if err != nil {
		log.L().Warn("ProxyImage:The new request failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	//伪装浏览器请求头
	// 关键：仿真浏览器请求头（对方可能根据 Referer 判断）
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118 Safari/537.36") //  风控/灰度-兼容/降级-用主流桌面 Chrome 的 UA 更像真实用户请求，提高放行率
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")                                                            // 告诉服务器：我能接受哪些 MIME 类型
	// 推荐设置 Referer 指向该 host 的根或页面（很多腾讯/QQ CDN 会检查此字段）
	// 例如：https://mat1.gtimg.com/ 或者 https://tianqi.qq.com/ 等，按对方要求调整
	req.Header.Set("Referer", "https://mat1.gtimg.com/") // 告诉上游“我从哪个页面跳过来”,很多 CDN（尤其腾讯/QQ 系）会根据 Referer 做防盗链校验

	resp, err := client.Do(req) // 执行请求
	if err != nil {
		log.L().Warn("ProxyImage: fetch remote image failed", zap.String("url", raw), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "fetch failed", "detail": err.Error()})
		return
	}

	defer resp.Body.Close() //最后响应体关闭
	if resp.StatusCode != http.StatusOK {
		log.L().Warn("ProxyImage: remote returned non-200", zap.Int("status", resp.StatusCode), zap.String("url", raw))
		// 将远端错误映射给客户端（可选更友好）
		c.JSON(http.StatusBadGateway, gin.H{"error": "remote returned non-200", "status": resp.StatusCode})
		return
	}

	// 透传 content-type - 只获得图片类型的所有图片
	ct := resp.Header.Get("Content-Type") // 拿到服务器给定的资源类型
	if ct == "" {                         // 为空就设置为通配类型
		ct = "image/*"
	}
	c.Header("Content-Type", ct) // 写入头部
	// 缓存一小时，减少后端压力
	c.Header("Cache-Control", "public, max-age=3600")       // 告诉浏览器与中间缓存（CDN/代理）：这个响应可被共享缓存（public），并且最多缓存 3600 秒（1 小时）。
	if _, err := io.Copy(c.Writer, resp.Body); err != nil { // resp.Body将上游返回的字节流；c.Writer 是写回给浏览器的输出流。
		log.L().Warn("copy to response failed", zap.Error(err))
	}
}
