package main

/*
(1) render array
$ curl http://localhost:9999/date
<html>
<body>
    <p>hello, gee</p>
    <p>Date: 2019-08-17</p>
</body>
</html>
*/

/*
(2) custom render function
$ curl http://localhost:9999/students
<html>
<body>
    <p>hello, gee</p>
    <p>0: Geektutu is 20 years old</p>
    <p>1: Jack is 22 years old</p>
</body>
</html>
*/

/*
(3) serve static files
$ curl http://localhost:9999/assets/css/geektutu.css
p {
    color: orange;
    font-weight: 700;
    font-size: 20px;
}
*/

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"gee"
)

type student struct {
	Name string
	Age  int8
}

func FormatAsDate(t time.Time) string {  //传入时间
	year, month, day := t.Date()
	return fmt.Sprintf("%d-%02d-%02d", year, month, day)
}
func time_now() gee.HandlerFunc{
    return func(c *gee.Context){
		now := time.Now()
		fmt.Println("已经使用当前的time模块"+now.Format("2006-01-02 15:04"))
	}
}
func main() {
	r := gee.New()
	r.Use(gee.Logger())  //这个是创建对象的根函数
	r.SetFuncMap(template.FuncMap{  //内部是模板函数表
		"FormatAsDate": FormatAsDate,
	})  //添加到这个模板函数里，之后一并使用
	//这里会将其所有的文件解析并放到同一个模板集合里-解析后，每个文件的基础文件名（不含路径）就是模板名
	r.LoadHTMLGlob("templates/*")  // 通配符加载并解析模板，扫描当前文件下的模板文件

	r.Static("/assets", "./static")  // 第一个url的相对路径，第二个文件内部的路径

	stu1 := &student{Name: "Geektutu", Age: 21}
	stu2 := &student{Name: "Jack", Age: 22}
	// 按照解析里查找
	r.GET("/", func(c *gee.Context) {  // 这里初始的界面是这样
		c.HTML(http.StatusOK, "css.tmpl", nil)
	})
	r.GET("/students", func(c *gee.Context) {
		c.HTML(http.StatusOK, "arr.tmpl", gee.H{ // 哈希表
			"title":  "gee_test",
			"stuArr": [2]*student{stu1, stu2},
		})
	})

	r.GET("/date", func(c *gee.Context) {
		c.HTML(http.StatusOK, "custom_func.tmpl", gee.H{
			"title": "gee_time",
			"now":   time.Date(2025, 10, 3, 0, 0, 0, 0, time.UTC),  //这里依据模板里的函数调用
		})
	})
    
	t := r.Group("/time")
	t.Use(time_now()) 
	{
		t.GET("/hello/:name", func(c *gee.Context) {
			// expect /hello/geektutu
			c.String(http.StatusOK, "hello %s, you're at %s\n", c.Param("name"), c.Path)
		})
	}
	r.Run(":9999")
}
