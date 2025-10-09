### DAY2
这里主要还是Day01的补充，之后的将其封装以及加入了POST功能是本节的重点，一定要学会POST方法的使用，并且要理解封装的原理
此外各种数据结构的使用以及各种接口的实现、指针参数要要理解
### 综述
首先，r := gee.New()  //创建gee对象实例
我们将下述映射传入到r的Router里,并且给了它一个路由规则,这里的的context是一个封装好的结构体，其中包含了响应的writer和请求的request，并且这两个参数都是自动激活的，不需要我们手动去调用
```go
	r.GET("/", func(c *gee.Context) {  //这里相比于Day1的函数传入的参数不一样func(w http.ResponseWriter, req *http.Request) 这里是传入了一个context对象即结构体，其中第一个参数是响应的writer,第二个参数是请求request，这两个参数都封装在context对象中了，并且都是打开网页自动激活的，不需要我们手动去调用
		c.HTML(http.StatusOK, "<h1>Hello Gee</h1>")
	})
	r.GET("/hello", func(c *gee.Context) {
		// expect /hello?name=geektutu
		c.String(http.StatusOK, "hello %s, you're at %s\n", c.Query("name"), c.Path)
	})

	r.POST("/login", func(c *gee.Context) {
		c.JSON(http.StatusOK, gee.H{
			"username": c.PostForm("username"),
			"password": c.PostForm("password"),
		})
	})

```
实际上的操作就是我们先写入路由表的函数，之后依据我们访问链接的参数来执行对应路由表的函数
这里封装的context对象，其中包含了响应的writer和请求的request，也包含了许多的参数，这里的函数如果没有找到就会发现报错，找不到路由表的函数对应404,代码如下
```go
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
		c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
	}
}
```
封装的原先的req *http.Request并且包含更多的参数，可以借此设置各种响应方法及响应体的各种文本和状态及参数。实际本质是封装和自定义写好了**各种请求对应的响应方法及响应体**

Gee是写入基本的请求方法，Get、Post以及RUN,并且封装了响应的方法，String、JSON、HTML等，并且封装了响应的writer和请求的request，也包含了许多的参数，这里的函数如果没有找到就会发现报错，找不到路由表的函数对应404

此外这里的c.Req.FormValue(key)对应如下:  
FormValue(key) 是 Go 标准库提供的方法：从请求体（POST、PUT）里读取 key=value 的表单字段
POST body: username=wxy&password=123456
c.PostForm("username")  // 返回 "wxy"
c.PostForm("password")  // 返回 "123456"
而注册路由是构建了一个函数映射表，通过路由规则和请求方法来查找对应的函数，并执行这个函数，这个函数是返回对应的json响应，将这个json数据返回给我们的客户端即浏览器进行解析
```go
r.POST("/login", func(c *gee.Context) {  
	c.JSON(http.StatusOK, gee.H{
		"username": c.PostForm("username"),
		"password": c.PostForm("password"),
	})
})
```

***注**:
1. values ...interface{} 空接口类型，可以存放 任意类型的值,而... 表示 可变参数，可以传入任意个参数，也可以不传参数。
2. format 是 格式化模板，类似 C 语言的 printf %s,例如 %s 字符串文本
c.Writer.Write([]byte(fmt.Sprintf(format, values...)))  // 这里的fmt.Sprintf是格式化字符串并返回字符串-然后转换成字节切片写入响应体中
3. 一个完整的URL:http://example.com/search?q=golang&page=2, 而?后面部分就是查询字符串（query string） 查询参数用 & 分隔，每个参数是 key=value 形式,因此q = "golang"和page = "2"
而[请求体对象]c.Req.URL.Query() 返回的是返回 url.Values,是map[string][]string
因此结果如下:
q := c.Query("q")      // "golang"
p := c.Query("page")   // "2"
因此下面代码获得的是golang和2(key来决定)
```go
func (c *Context) Query(key string) string {  
	return c.Req.URL.Query().Get(key) //根据URL中的值获取查询以得到其参数
}
```
![alt text](image.png)
5. 
type H map[string]interface{} 是一个哈希表，但是将其转换为json的顺序是不确定的，最好对于json数据用结构体
type LoginResponse struct {
    Username string `json:"username"`
    Password string `json:"password"`  //这里是反射
}来写比较好
c.JSON(http.StatusOK, gee.H{ //这里的H是我们自定义的一个类型，是一个map,这之后会转为json格式。gee.H是一个哈希表无需的，所以最后的json是无序的 
1. c.SetHeader("Content-Type", "text/plain; charset=utf-8") 这里请求头的格式和内容用的是UTF-8编码，保证中文的正常输出
6.git 部分指令 
git log 查看提交记录，而q是推出
# 添加所有修改和新文件到暂存区
git add -A
git status 暂存区状态
git commit -m "Day02的学习" 信息带""