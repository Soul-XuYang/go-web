package gee

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// 哈希表的 key 是无序的 → 遍历或者编码时顺序随机
type H map[string]interface{} // type H是类型即别名 构建字典表，这里的接口不是Day1的类型函数，这里是空接口,可以存放任何值(对应类型函数)

type Context struct { // 构建结构体-实际上是对http.Request和http.ResponseWriter的封装，这样更方便我们操作-这里context就是一个上下文对象-请求对象
	// origin objects
	Writer http.ResponseWriter
	Req    *http.Request
	// request info
	Path   string
	Method string
	// response info
	StatusCode int
}

// 讲对应的参数传入到我们的context对象中
func newContext(w http.ResponseWriter, req *http.Request) *Context { // 第二个是请求对象，第一个是返回的响应对象
	return &Context{ //构建一个新的context对象
		Writer: w,
		Req:    req,
		Path:   req.URL.Path,
		Method: req.Method,
	}
}

// 下面是构建映射表
// 这里的Req是一个结构体指针，FormValue是其方法-对应的请求方法
func (c *Context) PostForm(key string) string {
	return c.Req.FormValue(key) //FormValue(key) 是 Go 标准库提供的方法
}

// 获取 URL 查询参数（GET 请求的 ?key=value) - 根据URL查询参数
func (c *Context) Query(key string) string {
	return c.Req.URL.Query().Get(key) //根据URL中的值获取查询以得到其参数
}

// PostForm/Query = 都是读入请求的参数
// 下面是写入响应的参数
func (c *Context) Status(code int) { // 设置状态码 例如404
	c.StatusCode = code        // 根据传入的code值设置其状态码,这样吸入响应体，它会自动发送200状态码，认为你是对的，但是我们实际需要考虑多种情况
	c.Writer.WriteHeader(code) // 必须在写入响应体之前调入WriteHeader写下状态码
}

func (c *Context) SetHeader(key string, value string) { // 设置响应对象的响应头-封装在context对象
	c.Writer.Header().Set(key, value)
}

// 设置响应体的各种文本和状态及参数
func (c *Context) String(code int, format string, values ...interface{}) { // 这里的...表示可变参数-可以接受任意参数
	c.SetHeader("Content-Type", "text/plain; charset=utf-8") //设置响应头告诉浏览器这是文本
	c.Status(code)                                           // 设置状态码
	// format 是 格式化模板，类似 C 语言的 printf %s,例如 %s 字符串文本
	c.Writer.Write([]byte(fmt.Sprintf(format, values...))) // 这里的fmt.Sprintf是格式化字符串并返回字符串-然后转换成字节切片写入响应体中
}

func (c *Context) JSON(code int, obj interface{}) { // 设置响应体的json格式-obj是任意模式
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer) //如果编码失败就返回500错误
	// Encode 方法会将 obj 编码为 JSON 格式，并将结果写入到 c.Writer 中
	// 如果编码过程中发生错误，Encode 方法会返回一个非 nil 的错误值
	// 这里我们检查这个错误值，如果不为 nil，就使用 http.Error 函数向客户端发送一个 500 状态码的错误响应，错误信息是 err.Error()
	// 这样做可以确保客户端在发生服务器端错误时能够收到适当的错误信息
	// 500 状态码表示服务器内部错误，通常用于指示服务器在处理请求时遇到了意外情况
	if err := encoder.Encode(obj); err != nil { // err接受Encode的返回值
		http.Error(c.Writer, err.Error(), 500)
	}
}

func (c *Context) Data(code int, data []byte) { //任意二进制数据：图片、视频、文件、压缩包
	c.Status(code)
	c.Writer.Write(data)
}

func (c *Context) HTML(code int, html string) { // 设置响应体的html格式
	c.SetHeader("Content-Type", "text/html")
	c.Status(code)
	c.Writer.Write([]byte(html)) //将html字符串转换成字节切片写入响应体中
}
