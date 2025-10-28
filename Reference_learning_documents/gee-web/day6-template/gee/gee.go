package gee

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
)


type HandlerFunc func(*Context)

// Engine implement the interface of ServeHTTP
type (
	RouterGroup struct {
		prefix      string
		middlewares []HandlerFunc // support middleware
		parent      *RouterGroup  // support nesting
		engine      *Engine       // all groups share a Engine instance
	}

	Engine struct {
		*RouterGroup
		router        *router
		groups        []*RouterGroup     // store all groups
		htmlTemplates *template.Template // 模板集合
		funcMap       template.FuncMap   // 模板函数表
	}
)

// New is the constructor of gee.Engine
func New() *Engine {
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

// Group is defined to create a new RouterGroup
// remember all groups share the same Engine instance
func (group *RouterGroup) Group(prefix string) *RouterGroup { //路由组的创建
	engine := group.engine
	newGroup := &RouterGroup{
		prefix: group.prefix + prefix,
		parent: group,
		engine: engine,
	}
	engine.groups = append(engine.groups, newGroup)
	return newGroup
}

// Use is defined to add middleware to the group
func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
	group.middlewares = append(group.middlewares, middlewares...)
}

func (group *RouterGroup) addRoute(method string, comp string, handler HandlerFunc) {
	pattern := group.prefix + comp
	log.Printf("Route: %4s - %s", method, pattern) // 打印日志信息
	group.engine.router.addRoute(method, pattern, handler)
}

// GET defines the method to add GET request
func (group *RouterGroup) GET(pattern string, handler HandlerFunc) {
	group.addRoute("GET", pattern, handler)
}

// POST defines the method to add POST request
func (group *RouterGroup) POST(pattern string, handler HandlerFunc) {
	group.addRoute("POST", pattern, handler)
}

// 创建静态文件处理端
func (group *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	relative_Path := path.Join(group.prefix, relativePath)             // 组的路径+实际的静态资源挂载点（相对路径）的路径吗
	fileServer := http.StripPrefix(relative_Path, http.FileServer(fs)) // StripPrefix(A, FileServer(B)) = 遇到以 A 开头的 URL，去掉 A，再用 B 这个文件系统去找对应文件返回。
	return func(c *Context) {
		file := c.Param("filepath")  // 获得filepath的值实际上是对应的剩余路径-这里对应的是文件路径
		if _, err := fs.Open(file); err != nil {  // 如果路径存在
			c.Status(http.StatusNotFound)  // 直接结束
            panic("文件不存在!")
		}
        // 从磁盘读取文件、设置合适的 Content-Type 并写回响应。
		fileServer.ServeHTTP(c.Writer, c.Req)  // 交给标准库文件服务器读取并回写
	}
}

// 大写暴露给用户的-给入网页路径和本地的文件路径
func (group *RouterGroup) Static(relativePath string, root string) {  //这里的把磁盘目录 root 封成一个 http.FileSystem所处理的目录
	handler := group.createStaticHandler(relativePath, http.Dir(root))  // 生成真正的处理函数
	urlPattern := path.Join(relativePath, "/*filepath")  // 拼成一个通配模式-动态访问文件路径
    // ✅ 注册 GET 路由到当前分组
    //    注意：最终路由 = group.prefix + urlPattern
    //    比如在 /api 组里调用 Static("/assets", ...)，最终是 /api/assets/*filepath
	
	// 这里是设立请求并添加对应的路由规则
	group.GET(urlPattern, handler)  // 给出对应的url路径和相应的处理函数(读取并返回/*filepath路径的静态文件数据)
}

// 设定对应的模板方法
func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
	// 创建一个空的模板集合
	// 把上一部设置的函数表注册进去（顺序很重要：必须在 Parse 之前）
	// 按通配符读取并解析文件（如 "templates/*"、"templates/**/*.tmpl"），把所有模板装进集合。
}

// Run defines the method to start a http server
func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middlewares []HandlerFunc
	for _, group := range engine.groups {
		if strings.HasPrefix(req.URL.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}
	}
	c := newContext(w, req)
	c.handlers = middlewares
	c.engine = engine
	engine.router.handle(c)
}
