package gee

import (
	"log"
	"time"
)

// 写了一个函数日志中间件
func Logger() HandlerFunc {
	return func(c *Context) {
		// Start timer
		t := time.Now() // 记录开始时间
		// Process request
		c.Next() // 放行，调用后续的中间件-如果有则继续没有就停止
		// Calculate resolution time
		log.Printf("Status_Code [%d] %s in %v", c.StatusCode, c.Req.RequestURI, time.Since(t)) //返回最后的状态码，请求路径，以及处理时间
	}
}

// c.Req.RequestURI
// 取的是 HTTP 请求行里的原始 URI（不含 scheme/host），也就是“路径 + 原始查询串”。
// 例：客户端发 GET /v1/users/42?verbose=1&sort=asc HTTP/1.1
// 则 RequestURI == "/v1/users/42?verbose=1&sort=asc"。
// 对比：

// c.Req.URL.Path == "/v1/users/42"

// c.Req.URL.RawQuery == "verbose=1&sort=asc"

// RequestURI 等于两者拼在一起（原样，不会被你修改过的参数影响）。

// time.Since(t)
// 计算“从 t 到现在”经过了多久，返回 time.Duration。
// 在这个中间件里，t := time.Now() 放在 c.Next() 之前，等到后续中间件和最终 handler 全部执行完再 time.Since(t)，就得到了整个请求处理链的总耗时（例如 12.4ms、1.3s）。
// Go 的 time 包含单调时钟成分，因此 Since 不受系统时间回拨/跳变影响，适合做耗时统计。
