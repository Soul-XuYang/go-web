package gee

import (
	"log"
	"net/http"
)

// HandlerFunc defines the request handler used by gee
type HandlerFunc func(*Context)

// 定义了两个结构体-对象引用环
type (
	RouterGroup struct {
    prefix      string           // 本组的路径前缀，例如 "/api"、"/api/v1"
    middlewares []HandlerFunc    // 作用在本组（及其子组）上的中间件链
    parent      *RouterGroup     // 父分组，支持嵌套（前缀叠加、中间件继承）
    engine      *Engine          // 指向所属的 Engine，便于组内注册路由时调用底层 router
	}

	Engine struct {
    *RouterGroup            // 匿名内嵌：把“根组”嵌进来，方法提升，直接 r.Group() 即可
    router *router          // 你的 Trie 路由器（按 method 分树）
    groups []*RouterGroup   // 收集所有分组，在 ServeHTTP 时按路径前缀聚合中间件
	}
)

// New is the constructor of gee.Engine
func New() *Engine {  //初始化其变量
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}


func (group *RouterGroup) Group(prefix string) *RouterGroup {
	engine := group.engine  //引擎一致
	newGroup := &RouterGroup{  // 在此条件下嵌套创建新的 RouterGroup
		prefix: group.prefix + prefix,  // 父组前缀+本次传入的前缀
		parent: group,  // 上述的父组
		engine: engine,  // 保存一致
	}
	engine.groups = append(engine.groups, newGroup)  // 每次创建新的 RouterGroup 都要把它收集起来，便于 ServeHTTP 时按路径前缀聚合中间件-进行拼接
	return newGroup 
}

func (group *RouterGroup) addRoute(method string, comp string, handler HandlerFunc) {
	pattern := group.prefix + comp  //拼接前缀和路径,这里的pattern是路由，handler是函数
	log.Printf("Route %4s - %s", method, pattern)  //打印日志
	group.engine.router.addRoute(method, pattern, handler)  // 调用底层router并向其路由表其函数
}

func (group *RouterGroup) GET(pattern string, handler HandlerFunc) {
	group.addRoute("GET", pattern, handler)
}

// POST defines the method to add POST request
func (group *RouterGroup) POST(pattern string, handler HandlerFunc) {
	group.addRoute("POST", pattern, handler)
}


func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)  //因为我这里给监听这里传入的是engine，所以这里engine需要实现ServeHTTP方法
}

// 注意这里，我们给 Engine 实现了 ServeHTTP 方法，这样我们就可以让 *Engine 实现了 http.Handler 接口。一旦来一次请求就会调用这个方法
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := newContext(w, req)  //构造上下文
	engine.router.handle(c)  //执行函数
}
