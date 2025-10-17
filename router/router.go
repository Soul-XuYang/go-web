package router

//路由组-分组
import (
	"project/controllers"
	"project/middlewares"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(middlewares.GinLogger(), middlewares.GinRecovery())
	mountSwagger(r)

	//加载数据
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/static", "./static") // 可选：放点 css/js
	//页面（公开）
	r.GET("/auth/login", func(c *gin.Context) { c.HTML(200, "login.html", nil) })
	r.GET("/auth/register", func(c *gin.Context) { c.HTML(200, "register.html", nil) })
	// r.GET("/dashboard", func(c *gin.Context) { c.HTML(200, "dashboard.html", nil) })
	auth := r.Group("/api/auth") //给出路由组的路径
	auth.POST("/login", controllers.Login)
	auth.POST("/register", controllers.Register)
	auth.POST("/logout", controllers.Logout)

	// 受保护的页面端
	page := r.Group("/page", middlewares.AuthMiddleWare()) //也是需要登录
	{
		page.GET("/shell", controllers.ShellPage)
		page.GET("/rates", func(c *gin.Context) { c.HTML(200, "exchange_rates.html", nil) })
		page.GET("/rmb-top10", func(c *gin.Context) { c.HTML(200, "rmb_top10.html", nil) })
		//文章界面
		page.GET("/articles", func(c *gin.Context) { c.HTML(200, "articles_pages.html", nil) })
		page.GET("/game/guess", func(c *gin.Context) { c.HTML(200, "game_guess_number.html", nil) })
	}

	// 受保护的 API（数据接口，需要登录）
	api := r.Group("/api", middlewares.AuthMiddleWare())
	{
		// 基本信息获取模块
		api.GET("/me", controllers.GetUserName)
		api.GET("/ad", controllers.Get_advertisement)

		// 汇率模块
		api.GET("/exchangeRates", controllers.GetExchangeRates)
		api.POST("/exchangeRates", controllers.CreateExchangeRate)
		api.POST("/rmb-top10/refresh", controllers.RefreshRmbTop10) // 手动刷新
		api.GET("/rmb-top10", controllers.GetRmbTop10)              // 读取快照

		//游戏模块
		api.POST("/game/guess", controllers.GameGuess)
		api.POST("/game/reset", controllers.GameGuess_Reset)
		//文章操作模块
		api.GET("/articles", controllers.Get_All_Articles)

	}
	
    // admin := r.Group("/admin", middlewares.RolePermission("admin","superadmin"))
	{
		
	}
	return r //返回路由组
}
