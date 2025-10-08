package gee

import (
	"log"
	"net/http"
	"strings"
)

// 定义了一个接口
type HandlerFunc func(*Context)  // 定义HandlerFunc类型，参数是Context类型，返回值是void

// 二者相互引用，所以需要先声明类型
type (
	RouterGroup struct {  // 这里是前置的分组列表
		prefix      string
		middlewares []HandlerFunc // support middleware
		parent      *RouterGroup  // support nesting
		engine      *Engine       // all groups share a Engine instance
	}

	Engine struct {
		*RouterGroup
		router *router // 对应的路由
		groups []*RouterGroup // 存有路由分组
	}
)


func New() *Engine {  // 新创建的Engine实例
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	log.Println("New Engine has created!!!")  
	return engine
}


func (group *RouterGroup) Group(prefix string) *RouterGroup { // 创建新的RouterGroup实例
	engine := group.engine
	newGroup := &RouterGroup{
		prefix: group.prefix + prefix,  //在其基础上拼接
		parent: group,
		engine: engine,
	}
	engine.groups = append(engine.groups, newGroup)
	return newGroup
}


// 这里是给分组（RouterGroup）“挂载中间件”的函数-这里组是在当前组下拼接的
func (group *RouterGroup) Use(middlewares ...HandlerFunc) {  //给分组（RouterGroup）“挂载中间件”
	group.middlewares = append(group.middlewares, middlewares...)  // 你调用 v1.Use(Auth(), RateLimit()) 时，这个函数把传入的**中间件函数**追加到该分组的 middlewares 切片里，仅做“登记”，不在此处执行
}

// 这里是对应路由的添加
func (group *RouterGroup) addRoute(method string, comp string, handler HandlerFunc) {
	pattern := group.prefix + comp
	log.Printf("Route %4s - %s", method, pattern) //日志的打印
	group.engine.router.addRoute(method, pattern, handler)  // 对应的添加路由
}

// 只有用到Get才会打印
func (group *RouterGroup) GET(pattern string, handler HandlerFunc) {
	group.addRoute("GET", pattern, handler)
}

// POST defines the method to add POST request
func (group *RouterGroup) POST(pattern string, handler HandlerFunc) {
	group.addRoute("POST", pattern, handler)
}

// Run defines the method to start a http server
func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)
}

//因为run了了http.ListenAndServe(addr, engine)，所以需要实现ServeHTTP方法
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {  
	var middlewares []HandlerFunc  // middlewares 切片，用于存储中间件函数
	// 一定是遍历所有的引擎下的分组，而不是遍历分组下的路由
	// 这里是查看所有的前缀是否存在对应一致的情况:URL和每组的前缀
	for _, group := range engine.groups {  // 遍历所有的路由分组-查询其请求的URL路径是否有对应的分组名称
		if strings.HasPrefix(req.URL.Path, group.prefix) {  //请求的路径是否有这个分组为开头
			middlewares = append(middlewares, group.middlewares...)  
		}
	}
	c := newContext(w, req)   // 创建Context上下文
	c.handlers = middlewares  // 将中间件函数赋值给Context.handlers
	engine.router.handle(c)   //执行这一函数
}
