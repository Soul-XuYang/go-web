package gee

import (
	"fmt"
	"log"
	"net/http"
)

// HandlerFunc 是一个类型，表示一个特定签名的函数
type HandlerFunc func(http.ResponseWriter, *http.Request) //定义一个函数
// ResponseWriter：用于构建 HTTP 响应的接口 //*Request：包含了请求信息的结构体。

// Engine implement the interface of ServeHTTP
type Engine struct { //包含一个路由表
	router map[string]HandlerFunc
}

// New is the constructor of gee.Engine
func New() *Engine { // 构造函数,返回值是其对象也可以认为是一个实例
	return &Engine{router: make(map[string]HandlerFunc)} //里面的就是创建这样一个实例
}

// 添加一个映射以及对应的函数并保存日志
func (engine *Engine) addRoute(method string, pattern string, handler HandlerFunc) {
	key := method + "-" + pattern                 //GET-/home
	log.Printf("Route %4s - %s", method, pattern) // Route GET - /home
	engine.router[key] = handler
}

// GET defines the method to add GET request
func (engine *Engine) GET(pattern string, handler HandlerFunc) {
	engine.addRoute("GET", pattern, handler)
}

// POST defines the method to add POST request
func (engine *Engine) POST(pattern string, handler HandlerFunc) {
	engine.addRoute("POST", pattern, handler)
}

// 运行启动一个服务器并监听端口,其由Engine来实现
func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine) //传入 http.ListenAndServe 的对象engine，这个是对第二个参数的要求:必须实现 ServeHTTP 方法
}

// ：ServeHTTP 函数在你打开链接时，Go 的 HTTP 服务器会自动 接受请求 并传递 w 和 req 参数给这个函数，然后根据这些参数执行相应的处理逻辑。
// 这个函数是 http.Handler 接口的实现，它处理传入的 HTTP 请求
// engine.ServeHTTP(w http.ResponseWriter, req *http.Request) 方法会被自动调用，w 和 req 参数是由 Go 的 HTTP 服务器在接收到请求时自动传递给你的 ServeHTTP 方法  // w 响应写入器，用于构建和发送 HTTP 响应给客户端 ，一个请求对象，包含了关于当前 HTTP 请求的所有信息
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) { //实现-即绑定,其中w是io.Writer接口,req是一个结构体
	//一旦有上述的参数传入,自动监听执行对应的ServeHTTP方法
	if req.URL.Path != "/favicon.ico" {
		fmt.Println("有人访问了", req.URL.Path, "路径!")
	}
	key := req.Method + "-" + req.URL.Path
	if handler, ok := engine.router[key]; ok { //如果存在这个映射，即原先的路由存有这个映射就执行我们所写的方法
		handler(w, req)
	} else {
		fmt.Fprintf(w, "404 NOT FOUND: %s\n", req.URL)
	}
}
