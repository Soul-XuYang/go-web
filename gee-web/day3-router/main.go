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
$ curl "http://localhost:9999/hello/geektutu"
hello geektutu, you're at /hello/geektutu

(4)
$ curl "http://localhost:9999/assets/css/geektutu.css"
{"filepath":"css/geektutu.css"}

(5)
$ curl "http://localhost:9999/xxx"
404 NOT FOUND: /xxx
*/

import (
	"gee"
	"net/http"
)

func main() {
	r := gee.New()                    //创建引擎
	r.GET("/", func(c *gee.Context) { // /hello?name=geektutu 是查询?后对应映射的名字
		c.HTML(http.StatusOK, "<h1>Hello Gee</h1>") //写入响应体返回html
	})

	r.GET("/hello", func(c *gee.Context) { //这个使用?查询的
		// expect /hello?name=geektutu
		c.String(http.StatusOK, "hello %s, you're at %s\n welcome to see you", c.Query("name"), c.Path)
	})

	r.GET("/hello/:name", func(c *gee.Context) { //第二个是直接映射的
		// expect /hello/geektutu
		c.String(http.StatusOK, "hello %s, you're at %s\n **Dynamic mapping**", c.Param("name"), c.Path)
	})

	r.POST("/login", func(c *gee.Context) { //POST请求-设置响应体的内容
		c.JSON(http.StatusOK, gee.H{ //这里的H是我们自定义的一个类型，是一个map,这之后会转为json格式。gee.H是一个哈希表无需的，所以最后的json是无序的
			"username": c.PostForm("username"),
			"password": c.PostForm("password"),
		})
	})

	r.PrintRoutes("GET")  // 打印路由信息
	r.PrintRoutes("POST") // 打印路由信息
	r.Run(":9999")
}
