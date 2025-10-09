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
	"log"
	"net/http"
)

// Engine is the uni handler for all requests
type Engine struct{} //

// ：ServeHTTP 函数在你打开链接时，Go 的 HTTP 服务器会自动 接受请求 并传递 w 和 req 参数给这个函数，然后根据这些参数执行相应的处理逻辑。
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) { //实现-即绑定
	    if req.URL.Path == "/favicon.ico" {
        return // 消除默认加载时的自动访问
    }
	switch req.URL.Path {
	case "/":
		fmt.Fprintf(w, "URL.Path = %q\n", req.URL.Path) //默认是get
		fmt.Printf("有人访问了%s路径!\n", req.URL.Path)
	case "/hello":
		var index int = 0
		for k, v := range req.Header {
			fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
			index++
		}
		fmt.Printf("有人访问了%s路径!\n", req.URL.Path)
		fmt.Fprintf(w, "循环已结束,共有%d个请求头\n", index)
	default: //其它路径
		fmt.Printf("有人访问了%s路径!\n", req.URL.Path)
		fmt.Fprintf(w, "404 NOT FOUND: http://localhost:9999/%s\n", req.URL)
	}
}

func main() {
	engine := new(Engine)
	//自动调用ServeHTTP方法
	fmt.Println("程序开始执行,其端口:9999")
	log.Fatal(http.ListenAndServe(":9999", engine)) // func ListenAndServe(address string, h Handler) error
}
