package router

//路由组-分组
import (
	"project/controllers"
	"project/middlewares"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(middlewares.GinLogger(), middlewares.GinRecovery()) //middlewares.GinLogger(),
	mountSwagger(r)

	//加载数据
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/static", "./static") // 可选：放点 css/js
	//页面（公开）
	r.GET("/auth/login", func(c *gin.Context) { c.HTML(200, "login.html", nil) })
	r.GET("/auth/register", func(c *gin.Context) { c.HTML(200, "register.html", nil) })
	auth := r.Group("/api/auth") //给出路由组的路径
	auth.POST("/login", controllers.Login)
	auth.POST("/register", controllers.Register)
	auth.POST("/logout", controllers.Logout)

	// 受保护的页面端
	page := r.Group("/page", middlewares.AuthMiddleWare()) //也是需要登录
	{
		page.GET("/shell", controllers.ShellPage) //界面
		page.GET("/rates", func(c *gin.Context) { c.HTML(200, "exchange_rates.html", nil) })
		page.GET("/rmb-top10", func(c *gin.Context) { c.HTML(200, "rmb_top10.html", nil) })
		//文章界面
		page.GET("/articles", func(c *gin.Context) { c.HTML(200, "articles_pages.html", nil) })
		// 游戏相关界面
		page.GET("/game/selection", func(c *gin.Context) { c.HTML(200, "game_selection.html", nil) })
		// 游戏界面
		page.GET("/game/guess", func(c *gin.Context) { c.HTML(200, "game_guess_number.html", nil) })
		page.GET("/game/map", func(c *gin.Context) { c.HTML(200, "game_map_time.html", nil) })
		page.GET("/game/map/display", func(c *gin.Context) { c.HTML(200, "map_display.html", nil) }) //可视化显示

		// game排行榜界面
		page.GET("/game/leaderboards", func(c *gin.Context) { c.HTML(200, "game_leaderboards.html", nil) })
		// 天气界面
		page.GET("/weather", func(c *gin.Context) { c.HTML(200, "weather.html", nil) })
		// 翻译界面
		page.GET("/translate", func(c *gin.Context) { c.HTML(200, "translator.html", nil) })
		page.GET("/translate/history", func(c *gin.Context) { c.HTML(200, "translator_history.html", nil) })

		// 计算器界面
		page.GET("/calculator", func(c *gin.Context) { c.HTML(200, "calculator.html", nil) })

		//文件管理界面
		page.GET("/files", func(c *gin.Context) { c.HTML(200, "upload.html", nil) })
		page.GET("/files/manage", func(c *gin.Context) { c.HTML(200, "file_lists.html", nil) })
	}

	// 受保护的 API（数据接口，需要登录）
	api := r.Group("/api", middlewares.AuthMiddleWare())
	{
		api.GET("/proxy/image", controllers.ProxyImage)

		// 基本信息获取模块
		api.GET("/me", controllers.GetUserName) //用户名称
		api.GET("/ad", controllers.Get_advertisement)

		// 汇率模块
		api.GET("/exchangeRates", controllers.GetExchangeRates)
		api.POST("/exchangeRates", controllers.CreateExchangeRate)
		api.POST("/rmb-top10/refresh", controllers.RefreshRmbTop10) // 手动刷新
		api.GET("/rmb-top10", controllers.GetRmbTop10)              // 读取快照

		// 天气信息模块
		weather := api.Group("/weather")
		weather.GET("/info", controllers.GetUser_Info)
		weather.GET("/top10", controllers.GetWeatherData_top10) // 获取 Top10 城市天气（返回数组）

		//游戏猜数字模块
		api.POST("/game/guess", controllers.GameGuess)
		api.POST("/game/reset", controllers.GameGuess_Reset)
		api.GET("/game/leaderboards", controllers.GameLeaderboards)
		api.GET("/game/leaderboard/me", controllers.GameLeaderboardMe) //获取个人排名和成绩-可以针对任何游戏
		// 地图游戏模块
		api.POST("/game/map/start", controllers.GameMapStart)       // 开始地图游戏
		api.POST("/game/map/complete", controllers.GameMapComplete) // 完成地图游戏
		api.POST("/game/map/reset", controllers.GameMapReset)       // 重置地图游戏
		api.GET("/game/map/display", controllers.Display_Map)       // 地图可视化界面
		//文章操作模块
		api.GET("/articles", controllers.Get_All_Articles)

		// 翻译功能模块
		api.POST("/translate", controllers.TranslateText)
		api.GET("/translate/languages", controllers.GetSupportedLanguages) //返给前端指定的翻译信息
		// 翻译历史记录模块
		api.GET("/translate/history", controllers.GetTranslationHistory)
		api.DELETE("/translate/history/:id", controllers.DeleteTranslationHistory) //删除指定的记录
		api.DELETE("/translate/history", controllers.ClearTranslationHistory)      //清空记录

		//文件资源管理系统
		api.POST("/files/upload", controllers.UploadFile)
		api.GET("/files/:id", controllers.DownloadFile) // Get只需要获得文件id即可
		api.DELETE("/files/:id", controllers.DeleteFile)
		api.GET("/files/lists", controllers.ListMyFiles)

		// 计算器模块
		api.POST("/calculator/calculate", controllers.Calculate)

	}
	// 超级管理员系统
	admin := r.Group("/admin", middlewares.RolePermission("admin", "superadmin")) //给定用户的身份登记
	{
		admin.GET("/dashboard", func(c *gin.Context) { c.HTML(200, "dashboard.html", nil) })
	}
	return r //返回路由组
}
