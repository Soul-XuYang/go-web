package gee

import (
	"log"
	"time"
)

func Logger() HandlerFunc {
	return func(c *Context) {
		// Start timer
		t := time.Now()
		// Process request
		c.Next() //处理请求-默认都会处理的
		// Calculate resolution time
		log.Printf("状态码:[%d] |URL:%s |运行的时间 %v", c.StatusCode, c.Req.RequestURI, time.Since(t))
	}
}
