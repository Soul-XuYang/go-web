package gee

import (
	"log"
	"net/http"
	"fmt"
)

// HandlerFunc defines the request handler used by gee
type HandlerFunc func(*Context)

// Engine implement the interface of ServeHTTP
type Engine struct { //自身带有一个路由表
	router *router
}

// New is the constructor of gee.Engine
func New() *Engine {
	return &Engine{router: newRouter()}
}

func (engine *Engine) addRoute(method string, pattern string, handler HandlerFunc) {
	log.Printf("Route %4s - %s", method, pattern)
	engine.router.addRoute(method, pattern, handler)
}

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
	return http.ListenAndServe(addr, engine)
}

func (e *Engine) PrintRoutes(method string) {
    for _, n := range e.router.getRoutes(method) {
        fmt.Println(n.String())
    }
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) { // 启动服务并加以接听
	c := newContext(w, req) // 创建上下文对象
	engine.router.handle(c)
}
