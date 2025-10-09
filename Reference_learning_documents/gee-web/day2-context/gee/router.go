package gee

import (
	"net/http"
)

type router struct { // 这里的router结构体存有一个映射表，key是method+path，value是HandlerFunc
	handlers map[string]HandlerFunc //这里的hanlders是一个映射表
}

func newRouter() *router {
	return &router{handlers: make(map[string]HandlerFunc)}
}

// 添加映射关系 routers指针
func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
	key := method + "-" + pattern
	r.handlers[key] = handler
}

// 根据method+path，这里使用rounter实现的指针，查找对应的handler
func (r *router) handle(c *Context) { //依据传入对象的路径和方法来查找对应的映射查找映射关系
	key := c.Method + "-" + c.Path
	// 这里handler是映射表,下面代码的意思就是哈希表中的key有对应的值，就执行这个函数，没有就返回404
	if handler, ok := r.handlers[key]; ok {  //如果存在这个映射(项目里我们有写这个函数的实现方法),就是我们之前已经写过这个函数的实现方法就认为是ok的 ,handler就是对应的函数
		handler(c)
	} else {
		c.String(http.StatusNotFound, "路由表没有找到对应的功能:404 NOT FOUND: %s\n", c.Path)
	}
}
