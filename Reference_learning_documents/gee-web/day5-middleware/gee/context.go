package gee

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type H map[string]interface{}

type Context struct { // 新增各种元素

	// origin objects - 上下文对象
	Writer http.ResponseWriter
	Req    *http.Request
	// request info - 请求信息
	Path   string
	Method string
	Params map[string]string
	// response info
	StatusCode int
	// middleware 中间件组
	handlers []HandlerFunc // 执行链中对应着中间组
	index    int           // 当前的执行位置的索引
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Path:   req.URL.Path,
		Method: req.Method,
		Req:    req,
		Writer: w,
		index:  -1, // 初始化为-1，表示还没有开始执行
	}
}

// 依次执行 handlers 中的函数
func (c *Context) Next() {
	c.index++ // 这里对应-1到0
	func_length := len(c.handlers)
	for ; c.index < func_length; c.index++ {
		c.handlers[c.index](c) // 执行对应的函数
	}
}

func (c *Context) Abort() {
	c.index = len(c.handlers)
	fmt.Println("Abort,已经提前终止对应的服务!")
} // 这个就是直接终止就行，无需写后面的响应、

func (c *Context) Fail(code int, err string) {
	c.index = len(c.handlers)      //末尾信息设置，终止当前的执行链，让NEXT函数不再执行
	if code >= 200 && code < 400 { // 默认编码404
		code = 404
		c.JSON(code, H{"message": err}) // code默认为401
	} else {
		panic("Fail:  The StatusCode is not in the range of 200-400!") //报出异常
	}
}

func (c *Context) Param(key string) string {
	value, _ := c.Params[key] // 返回对应参数的值
	return value
}

func (c *Context) PostForm(key string) string {
	return c.Req.FormValue(key)
}

func (c *Context) Query(key string) string {
	return c.Req.URL.Query().Get(key)
}

func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key string, value string) {
	c.Writer.Header().Set(key, value)
}

func (c *Context) String(code int, format string, values ...interface{}) {
	c.SetHeader("Content-Type", "text/plain")
	c.Status(code)
	c.Writer.Write([]byte(fmt.Sprintf(format, values...)))
}

func (c *Context) JSON(code int, obj interface{}) {
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(obj); err != nil {
		http.Error(c.Writer, err.Error(), 500)
	}
}

func (c *Context) Data(code int, data []byte) {
	c.Status(code)
	c.Writer.Write(data)
}

func (c *Context) HTML(code int, html string) {
	c.SetHeader("Content-Type", "text/html")
	c.Status(code)
	c.Writer.Write([]byte(html))
}
