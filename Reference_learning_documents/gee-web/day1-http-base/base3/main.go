package main

// $ curl http://localhost:9999/
// URL.Path = "/"
// $ curl http://localhost:9999/hello
// Header["Accept"] = ["*/*"]
// Header["User-Agent"] = ["curl/7.54.0"]
// curl http://localhost:9999/world
// 404 NOT FOUND: /world

import (
	"fmt"
	"net/http"

	"gee"
)

func main() {
	r := gee.New()
	fmt.Println("程序开始执行,其端口:9999")
	//写入对应的参数  一是各种的请求方法 二是各种URL对应的字符串,以下两个函数都是传入映射和相应函数操作(但是函数本身是没有执行的)
	r.GET("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "Hello Gee\n")
		fmt.Fprintf(w, "URL.Path = %q\n", req.URL.Path)
	})
	r.GET("/hello", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "请求头如下:\n")
		for k, v := range req.Header {
			fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
		}
	})
	fmt.Println("相关参数和映射以及传入，服务器已准备就绪!")
	// 直到这里准备就绪
	r.Run(":9999")
}
