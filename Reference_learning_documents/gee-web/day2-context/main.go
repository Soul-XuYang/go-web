package main

/*
(1)
$ curl -i http://localhost:9999/
HTTP/1.1 200 OK
Date: Mon, 12 Aug 2019 16:52:52 GMT
Content-Length: 18
Content-Type: text/html; charset=utf-8
<h1>Hello Gee</h1>

(2)
$ curl "http://localhost:9999/hello?name=geektutu"
hello geektutu, you're at /hello

(3)
$ curl "http://localhost:9999/login" -X POST -d 'username=geektutu&password=1234'
{"password":"1234","username":"geektutu"}

(4)
$ curl "http://localhost:9999/xxx"
404 NOT FOUND: /xxx
*/

import (
	"fmt"
	"net/http"

	"gee"
)

func main() {
	r := gee.New()                    //创建gee对象实例
	r.GET("/", func(c *gee.Context) { //这里相比于Day1的函数传入的参数不一样func(w http.ResponseWriter, req *http.Request) 这里是传入了一个context对象即结构体，其中第一个参数是响应的writer,第二个参数是请求request，这两个参数都封装在context对象中了，并且都是打开网页自动激活的，不需要我们手动去调用
		c.HTML(http.StatusOK, "<h1>Hello Gee</h1>") //设置状态码和响应体
	})
	r.GET("/hello", func(c *gee.Context) {
		// expect /hello?name=geektutu
		c.String(http.StatusOK, "hello %s, you're at %s\n I am %s to see you!", c.Query("name"), c.Path, c.Query("emotion")) //name，自定义
	})
	// 这里的请求需要我们使用html网页点击申请才行
	r.POST("/login", func(c *gee.Context) { //POST请求-设置响应体的内容
		c.JSON(http.StatusOK, gee.H{ //这里的H是我们自定义的一个类型，是一个map,这之后会转为json格式。gee.H是一个哈希表无需的，所以最后的json是无序的
			"username": c.PostForm("username"),
			"password": c.PostForm("password"),
		})
	})
	fmt.Println("服务器已运行!:http://localhost:9999")
	r.Run(":9999")
}
