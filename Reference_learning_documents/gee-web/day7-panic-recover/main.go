package main

import (
	"gee"
	"net/http"
)

func main() {
	r := gee.Default()
	r.GET("/", func(c *gee.Context) { //根目录
		c.String(http.StatusOK, "Hello Geektutu\n")
	})
	// index out of range for testing Recovery()
	r.GET("/panic", func(c *gee.Context) { //这里是测试路径
		names := []string{"gee", "abc"}     //只有两个元素
		c.String(http.StatusOK, names[100]) //报错
	})

	r.Run(":9999")
}
