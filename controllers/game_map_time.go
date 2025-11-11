package controllers

import (
	"container/heap"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"project/config"
	"project/global"
	"project/log"
	"project/models"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ***注意这里的后端程序在计算时或者耗时的程序时一定要加锁，否则会报错

var game_rounds = [3]int{8, 12, 16}                   // 不同难度对应的大小
var dir = [4][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} // 四个方向

const map_users_number = 10 // 每个用户最多保存5条记录

type P struct { // 坐标点-保持json映射关系
	X int `json:"x"`
	Y int `json:"y"`
}

// 地图游戏玩家状态-req数据
type MapGamePlayer struct {
	Round            int       `json:"round"`            // 当前轮次：1, 2, 3
	Difficulty       int       `json:"difficulty"`       // 当前轮次难度等级：0, 1, 2
	MapData          [][]byte  `json:"mapData"`          // 地图数据
	StartPoint       P         `json:"startPoint"`       // 起点
	EndPoint         P         `json:"endPoint"`         // 终点
	RoundStartTime   time.Time `json:"roundStartTime"`   // 当前轮开始时间
	GameStartTime    time.Time `json:"gameStartTime"`    // 整局游戏开始时间
	TotalTime        float64   `json:"totalTime"`        // 累计总时间（秒）
	IsRoundCompleted bool      `json:"isRoundCompleted"` // 当前轮是否完成
}

// 创建全局变量-mapGameState表-玩家表
type mapGameState struct {
	Players map[uint]*MapGamePlayer // id映射玩家表
	mu      sync.Mutex              // 互斥锁
}

var mapGame = &mapGameState{ // 创建全局变量-mapGameState表
	Players: make(map[uint]*MapGamePlayer),
}

// ---------------------------------------

// 初始化地图游戏玩家-重置
func init_MapGamePlayer() *MapGamePlayer { // 	初始化玩家状态-返回玩家指针
	return &MapGamePlayer{
		Round:            1,
		Difficulty:       0, // 第1轮从简单开始
		GameStartTime:    time.Now(),
		RoundStartTime:   time.Now(),
		TotalTime:        0,
		IsRoundCompleted: false,
	}
}

// 根据轮次获取对应的难度
func getDifficultyForRound(round int) int {
	if round <= 0 || round > 3 {
		log.L().Error("DifficultyForRound failed", zap.Int("round", round))
		return 0
	}
	return round - 1 // 第1轮=难度0，第2轮=难度1，第3轮=难度2
}

// 获取难度名称
func getDifficultyName(difficulty int) string {
	switch difficulty {
	case 0:
		return "简单"
	case 1:
		return "中等"
	case 2:
		return "困难"
	default:
		return "简单"
	}
}

/********* DTO数据-响应数据 *********/
// 易错，因为json会把byte[]映射成string，所以需要用[]string,因此需要改成strinf
type startMapGameResp struct {
	Message         string   `json:"message"`
	Round           int      `json:"round"`
	Difficulty      int      `json:"difficulty"`
	Size            int      `json:"size"`
	MapData         []string `json:"mapData"` // ← 改这里
	StartPoint      P        `json:"startPoint"`
	EndPoint        P        `json:"endPoint"`
	CurrentDistance int      `json:"currentDistance"`
	TotalTime       float64  `json:"totalTime"`
}

type completeMapGameResp struct { // 返回完成游戏的数据
	Message      string  `json:"message"`
	Round        int     `json:"round"`        // 当前轮次
	RoundTime    float64 `json:"roundTime"`    // 本轮用时（秒）
	TotalTime    float64 `json:"totalTime"`    // 累计总时间（秒）
	Saved        bool    `json:"saved"`        // 是否保存到数据库（仅第3轮）
	GameComplete bool    `json:"gameComplete"` // 是否完成全部三轮
}

type resetMapGameResp struct { // 返回重置游戏状态的数据
	Message string `json:"message"`
	Reset   bool   `json:"reset"`
}

// GameMapStart godoc
// @Summary     开始地图游戏
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Success     200  {object}  startMapGameResp  "响应数据"
// @Router      /api/game/map/start [post]
func GameMapStart(c *gin.Context) { //开始游戏
	// 不用请求参数-因为用户一点击按钮就开始游戏了
	uid := c.GetUint("user_id")
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	mapGame.mu.Lock() //加锁

	// 取/建玩家状态
	p, ok := mapGame.Players[uid] // 取玩家状态
	if !ok {
		p = init_MapGamePlayer() // 如果玩家不存在就初始化表
		mapGame.Players[uid] = p // 保存玩家状态-全局变量
	}
	// 根据当前轮次设置难度
	difficulty := getDifficultyForRound(p.Round) // 根据当前轮次设置难度-这里p.round是前端传来的
	p.Difficulty = difficulty                    // 保存难度-全局变量
	mapGame.mu.Unlock()                          // 解锁-以求用户进行按钮或者触发
	// 生成地图
	size := game_rounds[difficulty]
	arr := array_init(size, size)

	// 生成起点
	startPoint := P{}
	startPoint.X, startPoint.Y = start_index(arr)

	// 简化路径生成 - 直接生成足够的路径-依据难度升级计算
	randNumber := rand.Intn(difficulty + 1) //这里易错不能用0一定会出问题
	switch {
	case randNumber <= 1:
		go_next(arr, startPoint, size, difficulty) //difficulty保证我们的go_next肯定初始的步数不会出错
	case randNumber == 2:
		primMaze(arr, startPoint)
	case randNumber == 3:
		boolChess := make(map[P]bool, 0)
		generateMazeDFS(arr, startPoint, boolChess) // DFS算法（高难度）
	default:
		go_next(arr, startPoint, size, difficulty)
	}
	arr[startPoint.X][startPoint.Y] = '+'
	// 找到终点
	endPoint, currentDistance := end_index(arr, startPoint)
	arr[endPoint.X][endPoint.Y] = 'x' //终点标记为x

	// 调试：统计地图内容和打印
	pathCount := 0
	wallCount := 0
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			if arr[i][j] == 'o' {
				pathCount++
			} else if arr[i][j] == '#' {
				wallCount++
			}
		}
	}
	// 更新玩家状态
	mapGame.mu.Lock()
	p.MapData = arr               // 保存当前的地图数据-全局变量
	p.StartPoint = startPoint     // 保存起点-全局变量
	p.EndPoint = endPoint         // 保存终点-全局变量
	p.RoundStartTime = time.Now() // 保存当前轮开始时间-全局变量
	p.IsRoundCompleted = false    // 保存当前轮是否完成-全局变量
	mapGame.mu.Unlock()           // 解锁-以求用户进行按钮或者触发
	// 返回响应
	rows := make([]string, size)
	for i := 0; i < size; i++ {
		rows[i] = string(arr[i]) // []byte → string（UTF-8）
	}
	resp := startMapGameResp{
		Message:         fmt.Sprintf("地图游戏第 %d 轮已开始！难度：%s，地图大小：%dx%d", p.Round, getDifficultyName(difficulty), size, size),
		Round:           p.Round,
		Difficulty:      difficulty,
		Size:            size,
		MapData:         rows, // ← 用 rows
		StartPoint:      startPoint,
		EndPoint:        endPoint,
		CurrentDistance: currentDistance,
		TotalTime:       p.TotalTime,
	}
	c.JSON(http.StatusOK, resp)
}

// GameMapComplete godoc
// @Summary     完成地图游戏
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  completeMapGameResp  "响应数据"
// @Router      /api/game/map/complete [post]
func GameMapComplete(c *gin.Context) {
	uid := c.GetUint("user_id")
	uname := c.GetString("username")
	if uid == 0 || uname == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	mapGame.mu.Lock()
	player, ok := mapGame.Players[uid]  // 取玩家状态
	if !ok || player.IsRoundCompleted { //如果玩家不存在就初始化表
		mapGame.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "user not found or no active game found or round already completed"})
		return
	}

	// 计算本轮用时（秒）
	roundTime := time.Since(player.RoundStartTime).Seconds() // 按秒来计算
	player.TotalTime += roundTime                            // 累计总时间
	player.IsRoundCompleted = true                           //上一轮完成

	var resp completeMapGameResp

	if player.Round == 3 {
		// 第3轮完成，游戏结束
		finalTime := player.TotalTime
		saved := true
		if err := saveMapGameScore(uid, uname, finalTime); err != nil { // 显示MySql再是redis排行榜
			log.L().Error("saveMapGameScore failed", zap.Error(err))
			saved = false // 保存失败
		}
		log.L().Info("A user's data has saved!!!\n")
		resp = completeMapGameResp{
			Message:      fmt.Sprintf("恭喜完成第 3 轮！本轮用时：%.2f 秒。三轮总用时：%.2f 秒。已为您开启新的一局。", roundTime, finalTime),
			Round:        player.Round,
			RoundTime:    roundTime, //本轮时间
			TotalTime:    finalTime,
			Saved:        saved, //
			GameComplete: true,
		}

		// 重置玩家状态，开启新的一局
		mapGame.Players[uid] = init_MapGamePlayer()
		mapGame.mu.Unlock()
		c.JSON(http.StatusOK, resp)
		return
	}

	// 未完成全部三轮，进入下一轮
	nextRound := player.Round + 1
	nextDifficulty := getDifficultyForRound(nextRound) //标明难度

	resp = completeMapGameResp{
		Message:      fmt.Sprintf("恭喜完成第 %d 轮！本轮用时：%.2f 秒。进入第 %d 轮（%s难度）。当前总用时：%.2f 秒。", player.Round, roundTime, nextRound, getDifficultyName(nextDifficulty), player.TotalTime), // 返回消息
		Round:        player.Round,
		RoundTime:    roundTime,
		TotalTime:    player.TotalTime,
		Saved:        false,
		GameComplete: false,
	}

	// 更新玩家状态进入下一轮
	player.Round = nextRound
	player.IsRoundCompleted = false
	// 注意：不重置RoundStartTime，等下次调用GameMapStart时再设置

	mapGame.mu.Unlock()
	c.JSON(http.StatusOK, resp)
}

// GameMapReset godoc
// @Summary     重置地图游戏状态
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Success     200   {object}  resetMapGameResp  "响应数据"
// @Router      /api/game/map/reset [post]
func GameMapReset(c *gin.Context) { // 重置按钮-清空当前用户的游戏状态-注意清空要一个个来
	uid := c.GetUint("user_id")
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	mapGame.mu.Lock()
	delete(mapGame.Players, uid) // 只删除当前用户的游戏状态
	mapGame.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"message": "当前用户的地图游戏状态已重置，可以重新开始游戏",
		"reset": true})
}

const displayNum = 24

type DisplayResp struct {
	MapData    []string `json:"mapData"`
	StartPoint P        `json:"startPoint"`
	EndPoint   P        `json:"endPoint"`
	Ok         bool     `json:"ok"`
	Path       []P      `json:"path"`
}

// Display_Map godoc
// @Summary     地图游戏可视化展示 - 支持三种地图生成算法
// @Description 根据choice参数生成不同算法的地图并返回最优路径
// @Description choice=1: 递归路径生成算法（简单随机路径，适合初学者）
// @Description choice=2: Prim迷宫生成算法（中等难度，生成随机迷宫）
// @Description choice=3: DFS深度优先迷宫算法（困难难度，生成复杂迷宫）
// @Description 前端建议：使用三个按钮分别对应三种算法，按钮文字可为"简单路径"、"Prim迷宫"、"DFS迷宫"
// @Tags        Game
// @Security    Bearer
// @Produce     json
// @Param       choice  query     int  true  "地图生成算法选择: 1=递归路径, 2=Prim迷宫, 3=DFS迷宫"
// @Success     200     {object}  DisplayResp  "返回地图数据、起终点、最优路径"
// @Failure     400     {object}  map[string]string  "无效的choice参数"
// @Router      /api/game/map/display [get]
func Display_Map(c *gin.Context) {
	choice := strings.TrimSpace(c.Query("choice"))
	choiceInt, err := strconv.Atoi(choice)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid choice parameter"})
		return
	}
	grid := array_init(displayNum, displayNum+8)
	sx, sy := start_index(grid)
	startPoint := P{X: sx, Y: sy}
	visitedChess := make(map[P]bool, 0)
	// 根据choice选择不同的地图生成算法
	switch choiceInt {
	case 1:
		go_next(grid, startPoint, displayNum+8, 5)
	case 2:
		primMaze(grid, startPoint)
	case 3:
		generateMazeDFS(grid, startPoint, visitedChess)
	default:
		go_next(grid, startPoint, displayNum, 3)
	}
	endPoint, _ := end_index(grid, startPoint)
	grid[startPoint.X][startPoint.Y] = '+'
	grid[endPoint.X][endPoint.Y] = 'x'
	path, ok := AStar(grid, startPoint, endPoint)
	//  输出 rows（每行一个 string）-这里返回给前端的数据以字符串数组开始
	rows := make([]string, displayNum)
	for i := 0; i < displayNum; i++ {
		rows[i] = string(grid[i])
	}

	c.JSON(http.StatusOK, DisplayResp{
		MapData:    rows,
		StartPoint: startPoint,
		EndPoint:   endPoint,
		Ok:         ok,
		Path:       path, // 保留首尾的A*最优路径
	})
}

/********* 地图生成辅助函数 *********/
type item struct {
	p    P
	g, f int // g=已走代价,h是估计代价 f是总代价
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func manhattan(a, b P) int { return abs(a.X-b.X) + abs(a.Y-b.Y) }

type pq []*item //存有结构体地址
// heap包的Push/Pop函数内部会：
// 调用你实现的Push方法添加元素
// 自动调用Up/Down操作来维护堆的性质
// 在维护过程中会使用你实现的Less方法进行比较
// 所以虽然你的Push方法只是简单地append，但通过heap包的Push函数调用时，会自动完成堆的维护。这就是为什么需要实现这些接口方法的原因 - 它们是heap包内部用来维护堆结构的基础。
func (h pq) Len() int           { return len(h) }
func (h pq) Less(i, j int) bool { return h[i].f < h[j].f || (h[i].f == h[j].f && h[i].g > h[j].g) } //首先按f值升序排序（f值越小优先级越高），选择g值更大的节点（即更接近目标的路径）
func (h pq) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }                                         // 交换数据
func (h *pq) Push(x any)        { *h = append(*h, x.(*item)) }                                      // 对这个切片加入，将x化为Item类型
func (h *pq) Pop() any {
	old := *h
	x := old[len(old)-1]  // 获得其末尾元素
	*h = old[:len(old)-1] //保留前面的尾巴元素
	return x
}

func AStar(grid [][]byte, start, end P) ([]P, bool) {
	row_length, col_length := len(grid), len(grid[0])
	in := func(p P) bool { return p.X >= 0 && p.X < row_length && p.Y >= 0 && p.Y < col_length } // 这个是判断边界
	block := func(p P) bool { return grid[p.X][p.Y] == '#' }

	Priorqueue := &pq{}                                                    //创建一个pq队列-&赋值
	heap.Init(Priorqueue)                                                  //初始化其优先级队列
	heap.Push(Priorqueue, &item{p: start, g: 0, f: manhattan(start, end)}) // 放入初始化点

	gScore := map[P]int{start: 0} //存储起点到每个点的代价
	came := make(map[P]P)         // 存储前驱节点
	closed := make(map[P]bool)    //闭集

	for Priorqueue.Len() > 0 {
		cur := heap.Pop(Priorqueue).(*item) //eap.Pop() 函数返回的是 interface{} 类型,需要返回指针类型即节点
		u := cur.p

		if closed[u] { //如果处理过则跳过-防止类似BFS的重叠
			continue
		}
		if u == end { // 该点为最终点-开始记录路径
			path := []P{u}   // 创建一个初始节点
			for u != start { //开始遍历前驱节点
				u = came[u] // 不是则添加
				path = append(path, u)
			}
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 { // 反转这个节点
				path[i], path[j] = path[j], path[i]
			}
			return path, true // 判断找到终点
		}
		closed[u] = true // 标明这个点被处理过

		for _, d := range dir { //遍历四个方向
			v := P{u.X + d[0], u.Y + d[1]} // 邻居点
			if !in(v) || block(v) {        // 如果无效跳过
				continue
			}
			ng := gScore[u] + 1                    //当前的代价
			if g, ok := gScore[v]; !ok || ng < g { //！ok标明是第一次访问这个点或者值大于ng则需要更新了
				gScore[v] = ng
				came[v] = u                                                          //前驱节点,此刻v的前节点为u
				heap.Push(Priorqueue, &item{p: v, g: ng, f: ng + manhattan(v, end)}) //加入到队列中
			}
		}
	}
	return nil, false
}

// 初始化地图
func array_init(row int, col int) [][]byte {
	arr := make([][]byte, row) //指定外层切片
	for i := 0; i < row; i++ {
		arr[i] = make([]byte, col) // 按列创建内层的切片大小
	}
	for i := 0; i < row; i++ {
		for j := 0; j < col; j++ {
			arr[i][j] = '#'
		}
	}
	return arr
}

// 打印地图（调试用）
func print_array(arr [][]byte) {
	for i := 0; i < len(arr); i++ {
		for j := 0; j < len(arr[i]); j++ {
			fmt.Printf("%c ", arr[i][j])
		}
		fmt.Printf("\n")
	}
}

// 生成起点
func start_index(arr [][]byte) (int, int) {
	row := len(arr)
	col := len(arr[0])
	x := rand.Intn(row)
	y := rand.Intn(col)
	return x, y
}

// 递归生成路径 - 修改算法确保生成足够路径-这个是最简单的路径-EASY
func go_next(arr [][]byte, start_point P, step int, step_rand int) {
	if step <= 0 {
		return
	}
	for i := 0; i < len(dir); i++ {
		newX := start_point.X + dir[i][0]
		newY := start_point.Y + dir[i][1]
		if newX >= 0 && newX < len(arr) && newY >= 0 && newY < len(arr[0]) {
			if arr[newX][newY] == '#' {
				randnum := rand.Intn(3) //
				if randnum > 0 || step_rand > 0 {
					arr[newX][newY] = 'o'
					go_next(arr, P{newX, newY}, step-1, step_rand-1)
				}
			}
		}
		// 不满足条件则跳出
	}
	// 最后也是跳出
}

// media难度
func primMaze(arr [][]byte, start P) { //这里是因为权重都为1，所以先不设置最小堆
	arr[start.X][start.Y] = 'o' //起点设置
	walls := make([]P, 0) //这里是初始化待破墙的队列
	// 添加起始点周围的墙
	Point_addWalls := func(p P) { //构建进入点函数
		for i := 0; i < len(dir); i++ { //
			wallX := p.X + dir[i][0]
			wallY := p.Y + dir[i][1]
			if wallX >= 0 && wallX < len(arr)-1 && wallY >= 0 && wallY < len(arr[0])-1 { //这里是四周的点
				if arr[wallX][wallY] == '#' { //如果是'o'跳过
					walls = append(walls, P{wallX, wallY}) //存入数据
				}
			}
		}
	}
	Point_addWalls(start) //从起点开始生成-上述为初始化过程

	for len(walls) > 0 { //队列里的数据-只不过这是一个随机队列
		// 随机选一个墙
		idx := rand.Intn(len(walls)) //获取索引
		wall := walls[idx]
		walls = append(walls[:idx], walls[idx+1:]...) // 弹出数据，这里切片的第二位置的数据需要用...展开

		visitedCount := 0

		for i := 0; i < len(dir); i++ { // 遍历四个方向
			newX := wall.X + dir[i][0]
			newY := wall.Y + dir[i][1]

			// 判断新坐标是否在迷宫范围内
			if newX >= 0 && newX < len(arr) && newY >= 0 && newY < len(arr[0]) {
				// 如果是通道，则计数
				if arr[newX][newY] == 'o' {
					visitedCount++
				}
			}
		}
		// 关键逻辑：如果四个方向中只有一侧已访问，打通这堵墙-避免全破的情况-实际上这个是迷宫通道生成的关键
		if visitedCount == 1 {
			arr[wall.X][wall.Y] = 'o' // 打通当前墙壁
			Point_addWalls(wall)      // 将新访问的格子周围的墙壁加入队列
		}
	}
}

// hard难度
func generateMazeDFS(arr [][]byte, current P, visited map[P]bool) { //通路为'o'
	visited[current] = true //标明
	arr[current.X][current.Y] = 'o'
	//随机打乱方向
	directions := []int{0, 1, 2, 3}
	rand.Shuffle(len(directions), func(i, j int) {
		directions[i], directions[j] = directions[j], directions[i]
	}) //交换-借助交换打乱顺序

	for _, i := range directions { //四个方向BFS似的DFS
		// 每次跳2格（中间保留墙壁）,二者之间打破墙壁
		newX := current.X + dir[i][0]*2
		newY := current.Y + dir[i][1]*2

		if newX >= 0 && newX < len(arr) && newY >= 0 && newY < len(arr[0]) { //实际上递归的跳出条件 + visit
			next := P{newX, newY}
			if !visited[next] { //加一个访问过
				// 打通二者之间的墙
				wallX := current.X + dir[i][0]
				wallY := current.Y + dir[i][1]
				arr[wallX][wallY] = 'o'
				generateMazeDFS(arr, next, visited)
			}
		}
	}
}

// BFS 找到最远的终点
func end_index(arr [][]byte, start_point P) (end_point P, far int) {
	row, col := len(arr), len(arr[0])
	far = 0
	const p_dist = -1
	distMap := make([][]int, row)
	for i := 0; i < row; i++ {
		distMap[i] = make([]int, col)
		for j := range distMap[i] {
			distMap[i][j] = p_dist
		}
	}
	queue := make([]P, 0)
	queue_push := func(p P, distance int) {
		distMap[p.X][p.Y] = distance
		queue = append(queue, p)
	}
	queue_push(start_point, 0)

	for head := 0; head < len(queue); head++ {
		p := queue[head]
		d := distMap[p.X][p.Y]
		if d > far {
			far = d
			end_point = p
		}
		for i := 0; i < len(dir); i++ {
			newX := p.X + dir[i][0]
			newY := p.Y + dir[i][1]
			if newX >= 0 && newX < row && newY >= 0 && newY < col {
				if distMap[newX][newY] != p_dist {
					continue
				}
				if arr[newX][newY] == 'o' || arr[newX][newY] == '+' {
					distMap[newX][newY] = d + 1
					queue_push(P{newX, newY}, d+1)
				}
			}
		}
	}
	return end_point, far
}

/********* DB 辅助 *********/

// 保存地图游戏完成时间
func saveMapGameScore(uid uint, username string, timeSeconds float64) (err error) {
	if uid == 0 || username == "" || timeSeconds <= 0 {
		return fmt.Errorf("invalid save params: uid=%d username='%s' time=%.3f", uid, username, timeSeconds)
	}
	// 开始数据库事务
	tx := global.DB.Begin()
	if err = tx.Error; err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit().Error
		}
	}()

	// 1) 统计该用户已有记录数
	var cnt int64
	if err = tx.Model(&models.Game_Map_Time{}).
		Where("user_id = ?", uid).
		Count(&cnt).Error; err != nil { //统计数
		return
	}

	// 2) 未达上限，直接新增
	if cnt < map_users_number { //数据库用户的上线人数
		rec := models.Game_Map_Time{UserID: uid, Score: timeSeconds, UserName: username}
		if err = tx.Create(&rec).Error; err != nil {
			return
		}
		// 更新 Redis 排行榜
		_ = updateTop10FastestAfterDB(uid, username, timeSeconds)
		return
	}

	// 3) 达上限：找到用时最长的记录并更新 - slowest为最慢的用户
	var slowest models.Game_Map_Time
	if err = tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", uid).
		Order("created_at ASC, id ASC").First(&slowest).Error; err != nil { // 升序排序-这里按照创建时间升序

		if errors.Is(err, gorm.ErrRecordNotFound) {
			rec := models.Game_Map_Time{UserID: uid, Score: timeSeconds, UserName: username}
			err = tx.Create(&rec).Error
			if err == nil {
				_ = updateTop10FastestAfterDB(uid, username, timeSeconds)
			}
			return
		}
		return
	}

	now := time.Now()
	err = tx.Model(&slowest).Updates(map[string]interface{}{ // 更新最慢的用户表中的数据
		"score":      timeSeconds,
		"created_at": now,
		"username":   username,
	}).Error
	if err != nil {
		return
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	_ = updateTop10FastestAfterDB(uid, username, timeSeconds)

	return
}

/********* Redis 排行榜 *********/

// Lua 脚本更新排行榜（用时越短越好，分数越低越好） - 执行脚本
var luaUpdateTop10Fastest = redis.NewScript(`  
local key     = KEYS[1]
local hname   = KEYS[2]
local member  = ARGV[1]
local score   = tonumber(ARGV[2])
local topK    = tonumber(ARGV[3])
local uname   = ARGV[4]

local cur = redis.call('ZSCORE', key, member)
if (not cur) or (score < tonumber(cur)) then 
  redis.call('ZADD', key, score, member)
end

local n = redis.call('ZCARD', key)
if n > topK then
  redis.call('ZREMRANGEBYRANK', key, topK, -1)
end

if uname and uname ~= '' then
  redis.call('HSET', hname, member, uname)
end
return 1
`)

// 更新地图游戏排行榜（用时越短分数越低，排名越靠前）
func updateTop10FastestAfterDB(userID uint, username string, timeSeconds float64) error {
	if userID == 0 || timeSeconds < 0 || global.RedisDB == nil {
		return nil
	}
	member := strconv.FormatUint(uint64(userID), 10)
	_, err := luaUpdateTop10Fastest.Run(global.RedisDB,
		[]string{config.RedisKeyTop10FastestMap, config.RedisKeyUsernames}, // key2和key2
		member,      // ARGV[1]
		timeSeconds, // ARGV[2] - 用时（秒）
		10,          // ARGV[3] TopK=10
		username,    // ARGV[4]
	).Result()
	return err
}
