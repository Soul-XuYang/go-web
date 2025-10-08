package gee

import (
	"log"
	"net/http"
)


type HandlerFunc func(*Context)  // 引入对象context结构体

// 里面存有router指针
type Engine struct {
	router *router //路由表
}

// New is the constructor of gee.Engine
func New() *Engine {  //创建一个gee类-结构体
	return &Engine{router: newRouter()}  //这里的router是一个指针，指向router结构体
} 

func (engine *Engine) addRoute(method string, pattern string, handler HandlerFunc) {
	log.Printf("Route %4s - %s", method, pattern)  //日志打印加载的路由
	engine.router.addRoute(method, pattern, handler)
}

// 以下操作都是实现HTTP方法，这里与day1无任何区别，只是第二个函数的传入参数变了是context对象
// GET defines the method to add GET request
func (engine *Engine) GET(pattern string, handler HandlerFunc) {
	engine.addRoute("GET", pattern, handler)
}

// POST defines the method to add POST request
func (engine *Engine) POST(pattern string, handler HandlerFunc) {
	engine.addRoute("POST", pattern, handler)
}

// Run defines the method to start a http server
func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)  //传入 http.ListenAndServe 的对象，必须实现 ServeHTTP 方法
}

// ServeHTTP 实现了 http.Handler 接口
// 这里是对ServeHTTP的重写
// ServeHTTP 处理所有的 HTTP 请求
// w 是响应写入器，用于构建和发送 HTTP 响应给客户端
// req 是一个请求对象，包含了关于当前 HTTP 请求的所有信息
// 这个函数会在每次有 HTTP 请求到达时被调用
// 伪代码：http 服务器内部逻辑
// for each incoming request {
//      1. 解析请求
//      2. 查找对应的处理器（handler）
//      3. 调用 handler.ServeHTTP(w, r)  // ← 就是这里！
// }
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := newContext(w, req)
	engine.router.handle(c)
}
