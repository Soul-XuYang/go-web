package controllers

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"project/config"
	"project/log"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket" //使用websocket
	"go.uber.org/zap"
)

type terminalMessage struct { //响应信息
	Type      string    `json:"type"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type terminalRequest struct { //请求
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	LineChoice bool     `json:"lineChoice"`
}

var (
	terminalUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024, //读缓冲区大小 B
		WriteBufferSize: 1024, //写缓冲区大小 B
		CheckOrigin: func(r *http.Request) bool { //检查请求头中的字段
			origin := r.Header.Get("Origin")
			return origin == "" || strings.Contains(origin, r.Host) //确保请求来自同一个主机
		},
	}
	//目前只支持linux指令
	allowedCommands = map[string]struct{}{
		"uptime":   {}, // 显示系统运行时间
		"top":      {}, // 显示进程信息
		"df":       {}, // 显示磁盘使用情况
		"free":     {}, // 显示内存使用情况
		"ps":       {}, // 显示当前进程
		"whoami":   {}, // 显示当前登录用户
		"ls":       {}, // 显示当前目录文件
		"tree":     {}, // 显示目录树
		"pwd":      {}, // 显示当前工作目录
		"cat":      {}, // 显示文件内容
		"echo":     {}, // 输出字符串
		"file":     {}, // 显示文件信息
		"stat":     {}, // 显示文件状态
		"date":     {}, // 显示当前日期和时间
		"hostname": {}, // 显示主机名"
		"uname":    {}, // 系统信息（uname -a）
		"ifconfig": {}, // 显示网络接口信息"
		"ping":     {}, // 测试网络连通性
		"lscpu":    {}, // 显示CPU信息"
		"lsmem":    {}, // 显示内存信息"
		"lsblk":    {}, // 显示块设备信息（磁盘分区）
		"clear":    {}, // 清楚打印信息
	}
	customCommands = map[string]string{
		"/help":    helpMessage,
		"/version": config.Version,
		"/stop":    "Stopping current CommandByLine",
	}
	// 后续待开发
	commonCommands = map[string]map[string]string{
		"uptime": {
			"linux":   "uptime",
			"windows": "systeminfo | find \"系统启动时间\"", // 或者 wmic os get lastbootuptime
			"darwin":  "uptime",
		},
		"top": {
			"linux":   "top",
			"windows": "tasklist",
			"darwin":  "top",
		},
		"df": {
			"linux":   "df -h",
			"windows": "wmic logicaldisk get size,freespace,caption",
			"darwin":  "df -h",
		},
		"free": {
			"linux":   "free -h",
			"windows": "wmic OS get FreePhysicalMemory,TotalVisibleMemorySize",
			"darwin":  "vm_stat",
		},
		"ps": {
			"linux":   "ps aux",
			"windows": "tasklist",
			"darwin":  "ps aux",
		},
		"who": {
			"linux":   "who",
			"windows": "query user",
			"darwin":  "who",
		},
		"ls": {
			"linux":   "ls",
			"windows": "dir",
			"darwin":  "ls",
		},
		"pwd": {
			"linux":   "pwd",
			"windows": "cd",
			"darwin":  "pwd",
		},
		"cat": {
			"linux":   "cat",
			"windows": "type",
			"darwin":  "cat",
		},
		"echo": {
			"linux":   "echo",
			"windows": "echo",
			"darwin":  "echo",
		},
		"clear": {
			"linux":   "clear",
			"windows": "cls",
			"darwin":  "clear",
		},
	}
	safeArgPattern = regexp.MustCompile(`^[\w\-/.:\p{Han}# ]+$`) //正则表达式-^表示字符串开始为必须以[]内的内容进行匹配,包括空格，\W是单词字符
	// \p{Han}是汉字，\w是单词字符，\-是减号，\./是点，:是冒号，#是井号 +是匹配前面的字符一次或多次 $表示字符串结束
	helpMessage = `Info:
System Information:
    uptime: Display system uptime
    hostname: Display the hostname
    uname: Display system information
    date: Display current date and time
    lscpu: Display CPU information
    lsmem: Display memory information
System Monitoring:
    top: Display process information
    df: Display disk usage
    free: Display memory usage
    ps: Display current processes
    lsblk: Display block device information
File Operations:
    ls: Display current directory contents
    tree: Display directory tree structure
    pwd: Display current working directory
    cat: Display file contents
    file: Display file type information
    stat: Display file status information
Network:
    ifconfig: Display network interface information
    ping: Test network connectivity
Utilities:
    whoami: Display currently system user
    echo: Output a string
    clear: Clear the terminal screen
	skip: Do nothing
Help:
    /help: Display this help message
    /version: Display the version of the server
  /stop: Stop the current CommandByLine
	
    ` //go中的多行字符串要用反引号
	// 锁和cmd的使用
	writeMu sync.Mutex //全局写入的锁-即每个用户进入那个循环的锁
)

// === 断开时机 ===
// 场景1：握手阶段断开
// 客户端发送HTTP升级请求后，在服务器响应前断开
// 这时连接还是HTTP连接，不会触发WebSocket的上下文关闭
// 场景2：WebSocket建立后断开
// 握手成功，WebSocket连接已建立
// 这时断开会触发WebSocket的检测机制

type terminalInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	GoVersion    string `json:"go_version"`
	Username     string `json:"username"`
}

// GetTerminalInfo 返回当前运行环境信息，仅允许超级管理员调用。
// GetTerminalInfo 获取终端信息
// @Summary 获取终端信息
// @Description 获取服务器终端的系统信息，包括操作系统、架构和Go版本
// @Tags System
// @Accept json
// @Produce json
// @Success 200 {object} terminalInfo
// @Failure 401 {object} map[string]string "no permission"
// @Router /api/admin/dashboard/terminal/info [get]
// @Security ApiKeyAuth
func GetTerminalInfo(c *gin.Context) {
	role := c.GetString("role")
	username := c.GetString("username")
	if role != "superadmin" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission"})
		return
	}
	c.JSON(http.StatusOK, &terminalInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		Username:     username,
	})
}

// TerminalWS 提供受控的命令行只读访问能力，负责升级连接、校验命令并将输出实时推送到前端。
// TerminalWS 提供基于WebSocket的终端交互接口
// @Summary WebSocket终端接口
// @Description 建立WebSocket连接，提供受控的命令行访问能力，支持实时命令执行和输出显示
// @Tags Terminal
// @Accept json
// @Produce json
// @Param role header string true "用户角色" default(superadmin)
// @Success 101 {string} string "WebSocket连接升级成功"
// @Failure 401 {object} map[string]string "无权限访问"
// @Failure 400 {object} map[string]string "WebSocket升级失败"
// @Router /api/admin/dashboard/terminal [get]
// @Security ApiKeyAuth
func TerminalWS(c *gin.Context) {
	role := c.GetString("role")

	if role != "superadmin" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission"})
		return
	}

	WebConnect, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil) //升级器函数-http响应写入器以及HTTP请求对象
	if err != nil {
		log.L().Error("failed to upgrade websocket",
			zap.Error(err),
			zap.String("remote_addr", c.Request.RemoteAddr),
			zap.String("user_agent", c.Request.UserAgent()),
		)
		return
	}
	defer WebConnect.Close()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	//读写锁-确保并发安全避免数据竞争
	send := func(msg terminalMessage) { //发送给前端数据加锁
		writeMu.Lock() //只有发送数据加锁防止并发竞争
		defer writeMu.Unlock()
		if err := WebConnect.WriteJSON(msg); err != nil {
			cancel()
			log.L().Warn("failed to write websocket message", zap.Error(err))
		}
	}
	// 初始信息
	// 这里不用地址数据是因为1不需要共享状态2信息不需要修改-不会出现值拷贝
	send(terminalMessage{
		Type:      "ready",
		Data:      "terminal websocket ready, send /help to search for all commands",
		Timestamp: time.Now(),
	})
	// ------------------------------

	// 单个用户控制自己面板的命令结构体
	type runningCommand struct {
		cancel context.CancelFunc //用于取消正在运行的函数
		done   chan struct{}      //用于通知函数已经完成
		name   string             //用于记录命令名称
	}
	var ( // 单个用户在自己websocket中的全局变量-用于控制for循环里的指令
		activeMu  sync.Mutex      //互斥锁保护公共即全局变量句柄的并发访问
		activeCmd *runningCommand //获取当前的命令的情况
	)
	stopActive := func(emitFeedback bool) {
		activeMu.Lock()
		current := activeCmd //立马获取当前的命令
		activeMu.Unlock()

		if current == nil { //获取当前的指令
			if emitFeedback {
				send(terminalMessage{
					Type:      "stdout",
					Data:      "No command need to stop now",
					Timestamp: time.Now(),
				})
			}
			return
		}
		current.cancel() //取消
		// 这里进程的终端处理很关键
		if current.done != nil {
			select {
			case <-current.done: //等待命令执行完成
			case <-time.After(3 * time.Second): //等待3秒-超时保护
			}
		}
		// 这里停止截止----------------------------------
		if emitFeedback { //是否发送停止信息
			send(terminalMessage{
				Type:      "stdout",
				Data:      "This command stopped successfully",
				Timestamp: time.Now(),
			})
		}

		// 重置全局变量
		activeMu.Lock()
		if activeCmd == current {
			activeCmd = nil
		}
		activeMu.Unlock()
	}
	//启动程序
	launchCommand := func(req terminalRequest) { //获得请求
		activeMu.Lock()
		if activeCmd != nil {
			activeMu.Unlock()
			send(terminalMessage{
				Type:      "status",
				Data:      "A command has already running. Use /stop to cancel this command first.",
				Timestamp: time.Now(),
			})
			return
		} //保护

		cmdCtx, cancelCmd := context.WithCancel(ctx) //借助上下文取消-这里
		running := &runningCommand{                  //取消程序的句柄
			cancel: cancelCmd,
			done:   make(chan struct{}),
			name:   strings.TrimSpace(req.Command),
		}
		activeCmd = running //赋值尽量要加锁
		activeMu.Unlock()
		// 这里开启一个进程
		go func(r *runningCommand, execReq terminalRequest, execCtx context.Context) {
			defer close(r.done) //关闭进程通道
			defer r.cancel()    //控制进程的关闭
			var err error
			if execReq.LineChoice {
				err = runTerminalCommandByline(execCtx, execReq, send)
			} else {
				err = runTerminalCommand(execCtx, execReq, send)
			}
			if err != nil && !errors.Is(err, context.Canceled) { //不是我们取消的错误
				send(terminalMessage{
					Type:      "error",
					Data:      err.Error(),
					Timestamp: time.Now(),
				})
			}
			// 执行完释放资源
			activeMu.Lock()
			if activeCmd == r {
				activeCmd = nil
			}
			activeMu.Unlock()
		}(running, req, cmdCtx)
	}

	commandChannel := make(chan terminalRequest, 20) //通道为请求数据,缓存为10

	// 开启一个线程-接受前端websocket发来的数据
	go func() {
		defer close(commandChannel) //持续收到前端发来的数据，最后要关闭
		for {
			// 持续读取 WebSocket 消息
			var req terminalRequest
			if err := WebConnect.ReadJSON(&req); err != nil { //接受请求的数据
				if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.L().Info("websocket connection closed by client")
				} else {
					log.L().Warn("error reading websocket message", zap.Error(err))
				}
				cancel() //断开
				return
			}
			if strings.TrimSpace(req.Command) == "" || strings.TrimSpace(req.Command) == "skip" { //不考虑这个情况
				continue
			}
			select {
			case commandChannel <- req:
			case <-ctx.Done():
				return
			}
		}
	}()

	//当前的后端线程发送响应的线程
	ticker := time.NewTicker(1 * time.Second) //1s计时器
	defer ticker.Stop()
	//主循环
	for {
		select {
		case <-ctx.Done():
			stopActive(false)
			return
		case <-ticker.C: //计时器每秒发送响应数据
			send(terminalMessage{
				Type:      "time", //通过type控制前端接受的数据
				Data:      time.Now().Format(time.RFC3339),
				Timestamp: time.Now(),
			})
		case req, ok := <-commandChannel: //如果命令不为空实际上就是通道接收到数据
			if !ok {
				log.L().Info("command channel closed")
				stopActive(false)
				return
			}
			if strings.TrimSpace(req.Command) == "" { //如果命令为空
				continue
			}
			if strings.HasPrefix(req.Command, "/") {
				if req.Command == "/stop" {
					stopActive(false)
					continue
				}
				if data, exists := customCommands[req.Command]; exists {
					send(terminalMessage{
						Type:      "stdout",
						Data:      data,
						Timestamp: time.Now(),
					})
					continue
				}
				send(terminalMessage{
					Type:      "error",
					Data:      fmt.Sprintf("Unknown command: %s", req.Command),
					Timestamp: time.Now(),
				})
				continue
			}
			if strings.TrimSpace(req.Command) == "clear" { //如果命令为clear 返回给前端要他情况
				send(terminalMessage{
					Type:      "clear",
					Data:      "",
					Timestamp: time.Now(),
				})
				continue
			}
			launchCommand(req) //新开一个线程保证不影响主程序
		} //select
	}
}

func runTerminalCommand(parentCtx context.Context, req terminalRequest, send func(terminalMessage)) error {
	cmdName := strings.TrimSpace(req.Command)
	if _, ok := allowedCommands[cmdName]; !ok {
		return fmt.Errorf("this command %q is not permitted", cmdName)
	}
	args := make([]string, 0, len(req.Args))
	for _, raw := range req.Args {
		arg := strings.TrimSpace(raw)
		if len(arg) > 100 {
			return fmt.Errorf("argument %q is too long", arg)
		}
		if arg == "" { //不需要读取空参数-跳过
			continue
		}
		if !safeArgPattern.MatchString(arg) {
			return fmt.Errorf("argument %q contains forbidden characters", arg)
		}
		args = append(args, arg)
	}
	ctx, cancel := context.WithTimeout(parentCtx, config.TerminalTTL)
	defer cancel()
	// 构建执行命令
	cmd := exec.CommandContext(ctx, cmdName, args...)
	var stdoutBuf, stderrBuf bytes.Buffer //缓冲区

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Start()
	if err != nil {
		log.L().Error("failed to start command", zap.Error(err))
		return fmt.Errorf("failed to start command: %w", err)
	}
	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded { //如果超时
			cmd.Process.Kill()
			return fmt.Errorf("command %q timed out", cmdName)
		}
		if parentCtx.Err() != nil { //如果上下文被取消
			return parentCtx.Err()
		}
		return fmt.Errorf("command execution failed: %w", err)
	}
	var wg sync.WaitGroup //命令完成后并发输出
	//双向通道
	wg.Add(1)
	go func() {
		defer wg.Done()
		if stdoutBuf.Len() > 0 {
			send(terminalMessage{
				Type:      "stdout",
				Data:      stdoutBuf.String(),
				Timestamp: time.Now(),
			})
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if stderrBuf.Len() > 0 {
			send(terminalMessage{
				Type:      "stderr",
				Data:      stderrBuf.String(),
				Timestamp: time.Now(),
			})
		}
	}()
	wg.Wait()
	return nil
}

// 按行输出命令结果
func runTerminalCommandByline(parentCtx context.Context, req terminalRequest, send func(terminalMessage)) error {
	cmdName := strings.TrimSpace(req.Command)
	if _, ok := allowedCommands[cmdName]; !ok { //判断是否可以运行
		return fmt.Errorf("this command %q is not permitted", cmdName)
	}
	// 获得前端的数据
	args := make([]string, 0, len(req.Args)) //这里args是参数
	// 遍历各个参数的值
	for _, raw := range req.Args { //遍历传来的参数切片-并一一校验
		arg := strings.TrimSpace(raw)
		if len(arg) > 100 {
			return fmt.Errorf("argument %q is too long", arg) //直接报错
		}
		if arg == "" {
			continue
		}
		if !safeArgPattern.MatchString(arg) {
			return fmt.Errorf("argument %q contains forbidden characters", arg)
		}
		args = append(args, arg)
	}

	ctx, cancel := context.WithTimeout(parentCtx, config.TerminalTTL)
	defer cancel() //后续记得关闭上下文

	cmd := exec.CommandContext(ctx, cmdName, args...) // 创建一个命令对象
	stdout, err := cmd.StdoutPipe()                   // 获取命令的标准输出管道
	if err != nil {
		return fmt.Errorf("failed to get stdoutPipe's data: %w", err)
	}
	stderr, err := cmd.StderrPipe() //获取命令的标准错误输出管道
	if err != nil {                 //获取错误失败
		return fmt.Errorf("failed to get stderrPipe's data: %w", err)
	}

	if err := cmd.Start(); err != nil { //启动命令
		return fmt.Errorf("failed to start command: %w", err)
	}

	var wg sync.WaitGroup
	output := make(chan terminalMessage, 64) // 64个通道-单个平均缓冲为32个数据

	stream := func(kind string, reader *bufio.Reader) { // 构建一个流函数:第一个表示流的信息-第二个表示读取的数据
		defer wg.Done()
		scanner := bufio.NewScanner(reader) // 创建一个新的 Scanner 来读取通道里的数据
		buf := make([]byte, 0, 64*1024)     //缓存区大小
		scanner.Buffer(buf, 512*1024)       // 最大缓存区大小

		for scanner.Scan() { // 持续读取数据-逐行读取数据
			output <- terminalMessage{ //将后端响应的数据发送给通道
				Type:      kind,
				Data:      scanner.Text(),
				Timestamp: time.Now(),
			}
		}
		//报错跳出循环-如果报错也发送对应的错误信息到通道里
		if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
			output <- terminalMessage{
				Type:      "error",
				Data:      fmt.Sprintf("%s stream  output error: %v", kind, err),
				Timestamp: time.Now(), //当前时间
			}
		}
	}

	wg.Add(2) //创建两个计数器-等待两个协程结束
	go stream("stdout", bufio.NewReader(stdout))
	go stream("stderr", bufio.NewReader(stderr))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

loop: //标签-跳出for循环
	for {
		select { //单个select只会等待并处理一个时间
		case <-parentCtx.Done():
			break loop
		case <-ctx.Done():
			break loop
		case msg, ok := <-output: //将通道的数据发送给给前端
			if !ok {
				break loop
			}
			send(msg)
		case <-done: //前方的两个计数器都结束了-实际上是两个通道管理了
			break loop
		}
	}

	close(output)             //跳出循环-关闭通道
	for msg := range output { //这里是将两个协程里缓存的所有数据都发送出去
		send(msg)
	}
	// 这里是前端的命令执行完了
	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded { //如果超时
			cmd.Process.Kill()
			return fmt.Errorf("command %q timed out", cmdName)
		}
		if parentCtx.Err() != nil { //如果上下文被取消
			return parentCtx.Err()
		}
		return fmt.Errorf("command execution failed: %w", err)
	}

	// send(terminalMessage{
	// 	Type:      "status",
	// 	Data:      fmt.Sprintf("command %q completed", cmdName),
	// 	Timestamp: time.Now(),
	// })
	return nil
}
