package gee

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
)

// 把当前调用栈堆栈（stack trace）取出来，拼成一段可读的“报错 + 回溯路径”的字符串
func trace(message string) string { //message是我们自定义的信息
	var pcs [32]uintptr             // 指令地址-32层
	n := runtime.Callers(3, pcs[:]) //  把当前 goroutine 的调用栈，按**返回地址（PC，program counter）**填进 pcSlice，返回实际写入的帧数 n

	var str strings.Builder
	str.WriteString(message + "\nTraceback:")
	for index, pc := range pcs[:n] { // range是切片范围遍历到n停止
		fn := runtime.FuncForPC(pc) // 映射成函数对象 - pc是程序计数器的地址，runtime.FuncForPC(pc) 会把这个地址映射成一个函数描述对象
		name := fn.Name()
		file, line := fn.FileLine(pc)
        // 统一式打印
		if(index == n -1){
        str.WriteString(fmt.Sprintf("filepath:%s |func_name:%s |line:%d", file, name, line))
		}else{                                                      
		str.WriteString(fmt.Sprintf("filepath:%s |func_name:%s |line:%d\n ", file, name, line)) // 打印行号和列号
		}
	}
	return str.String()
}

func Recovery() HandlerFunc {
	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil { // 有错误
				message := fmt.Sprintf("%s", err)  //报错错误原因
				log.Printf("%s", trace(message))  //给出信息
				c.Fail(http.StatusInternalServerError, "Internal Server Error")  //返回对应值
			}
		}()

		c.Next()
		fmt.Println("此刻程序执行完毕!")
	}
}
