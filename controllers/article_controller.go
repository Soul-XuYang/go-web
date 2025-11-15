package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"project/config"
	"project/global"
	"project/models"
	"project/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 创建对应的DTO
type CreateArticleDTO struct {
	Title   string `json:"title"   binding:"required,min=1,max=200"`
	Content string `json:"content" binding:"required"`
	Preview string `json:"preview" binding:"required"`
}

// 更新文章的请求 DTO —— 字段可选，表示“部分更新”
type UpdateArticleDTO struct {
	Title   *string `json:"title,omitempty"`
	Content *string `json:"content,omitempty"`
	Preview *string `json:"preview,omitempty"`
}

type ArticleResp struct { //DTO这里是给数据库要更改的数据
	UserName  string `json:"username"`
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	Preview   string `json:"preview"`
	Likes     uint   `json:"likes"`
	CreatedAt string `json:"created_at"`
}

type ArticleListResp struct {
	ID              uint   `json:"id"`
	Username        string `json:"username"`
	Title           string `json:"title"`
	Preview         string `json:"preview"`
	Likes           uint   `json:"likes"`
	Commentcount    uint   `json:"commentcount"`
	RepostCount     uint   `json:"repost_count"`
	CollectionCount uint   `json:"collection_count"`
}

// CreateArticle godoc
// @Summary      创建文章
// @Tags         Articles
// @Security     Bearer
// @Accept       json
// @Produce      json
// @Param        body  body      controllers.CreateArticleDTO  true  "创建文章参数"
// @Success      201   {object}  controllers.ArticleResp
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /create_articles [post]
func CreateArticle(c *gin.Context) {
	uid := c.GetUint("user_id") // 中间件放进去的当前用户ID
	uname := c.GetString("username")

	var input CreateArticleDTO                       //获取前端的数据                      // 创建DTO对象
	if err := c.ShouldBindJSON(&input); err != nil { //接受传来的数据对象
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将 DTO 映射到模型，服务端**显式**设置 UserID，避免前端伪造-获取到传来的数据
	art := models.Article{
		UserID:  uid,
		Title:   input.Title,
		Content: input.Content,
		Preview: input.Preview,
	}
	// 实际赋值还是找对应的赋值
	if err := global.DB.Create(&art).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 组织响应 DTO（不把敏感字段回给前端）-只返回对应的文章ID
	resp := ArticleResp{
		UserName: uname, ID: art.ID, Title: art.Title, Preview: art.Preview,
		Likes: art.Likes,
	}
	c.JSON(http.StatusCreated, resp) //响应数据-一定要有id之后的数据界面的url就是根据id打开的
}

// UpdateArticle godoc
// @Summary      更新文章数据
// @Tags         Articles
// @Security     Bearer
// @Accept       json
// @Produce      json
// @Param        id    path      string                        true  "文章ID"
// @Param        body  body      controllers.UpdateArticleDTO  true  "更新文章参数"
// @Success      200   {object}  controllers.ArticleResp
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /update_articles/{id} [put]
// PUT/PATCH请求-更新文章
func UpdateArticle(c *gin.Context) {
	user_id := c.GetUint("user_id") // 中间件放进去的当前登录用户ID,实际上前端管理文章用id就行
	idStr := c.Param("id")          // :id
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var input UpdateArticleDTO //接受前端的数据-各个修改点
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 只更新input的非 nil 的字段
	updates := map[string]interface{}{} // 创建空的哈希表
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Content != nil {
		updates["content"] = *input.Content
	}
	if input.Preview != nil {
		updates["preview"] = *input.Preview
	}

	// 只允许修改“我自己的那一行”
	tx := global.DB.Model(&models.Article{}).
		Where("id = ? AND user_id = ?", id, user_id).
		Updates(updates) // 会自动更新 UpdatedAt -这里既可以

	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tx.Error.Error()})
		return
	}
	if tx.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}

	// 修改完了返回更新后的数据（可选：再查一次）
	var out models.Article
	if err := global.DB.Where("id = ? AND user_id = ?", id, user_id).First(&out).Error; err != nil {
		// 正常不该失败；失败就返回 200+轻量确认
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	resp := ArticleResp{
		ID:        out.ID,
		UserName:  c.GetString("username"), // 或从关联 User 获取
		Title:     out.Title,
		Preview:   out.Preview,
		Likes:     out.Likes,
		CreatedAt: out.CreatedAt.Format(utils.FormatTime_specific),
		// 注意：Update 接口是否要返回 UpdatedAt？建议加
	}
	c.JSON(http.StatusOK, resp) //因为当今的RESTful 的 PUT / PATCH 响应 通常返回更新后的资源表示
}

// 获取当前系统的全部文章-论坛
// Get_All_Articles godoc
// @Summary      获取全部文章
// @Tags         Articles
// @Security     Bearer
// @Produce      json
// @Param        title            query  string false "关键字（匹配文件名，模糊）"
// @Param        page         query  int    false "页码（默认1）"
// @Param        page_size    query  int    false "每页的条数（默认10，最大100）"
// @Param        order        query  string false "排序：共8种组合，两种排序方式-上传日期 created_desc（默认）/created_asc/likes_desc/likes_asc/comments_desc/comments_asc/reposts_desc/reposts_asc/collections_desc/collections_asc"
// @Success      200  {array}   controllers.ArticleListResp
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /articles [get]
func Get_All_Articles(c *gin.Context) {
	title := strings.TrimSpace(c.Query("title"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	order := strings.TrimSpace(c.Query("order"))

	// 页数限制
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	// 是否使用缓存：仅限无搜索、第一页、默认排序-这里是无筛选是
	useCache := (title == "" && page == 1 && (order == "" || order == "created_desc"))
	cacheKey := config.RedisHomePage
	if useCache { //默认主页使用缓存
		var cachedItems []ArticleListResp
		if err := global.RedisDB.Get(cacheKey).Scan(&cachedItems); err == nil {
			c.JSON(http.StatusOK, cachedItems)
			return
		}
	}

	// 构建查询
	db := global.DB.Model(&models.Article{}).Where("deleted_at IS NULL") // 显式排除软删除
	if title != "" {
		db = db.Where("title LIKE ?", "%"+title+"%") //查询title
	}
	switch order {
	case "created_asc":
		db = db.Order("created_at ASC")
	case "likes_desc":
		db = db.Order("likes DESC")
	case "likes_asc":
		db = db.Order("likes ASC")
	case "comments_desc":
		db = db.Order("comment_count DESC")
	case "comments_asc":
		db = db.Order("comment_count ASC")
	case "reposts_desc":
		db = db.Order("repost_count DESC")
	case "reposts_asc":
		db = db.Order("repost_count ASC")
	case "collections_desc":
		db = db.Order("collection_count DESC")
	case "collections_asc":
		db = db.Order("collection_count ASC")
	default:
		db = db.Order("created_at DESC")
	}
	db = db.Select("id, user_id, title, preview, likes, repost_count, comment_count, collection_count, created_at, updated_at") // 只查询这几个字段
	var articles []models.Article
	if err := db.Preload("User", func(tx *gorm.DB) *gorm.DB {
		return tx.Select("id, username")
	}).Offset((page - 1) * size).Limit(size).Find(&articles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	items := make([]ArticleListResp, 0, len(articles))
	for _, a := range articles {
		items = append(items, ArticleListResp{
			ID:              a.ID,
			Username:        a.User.Username,
			Title:           a.Title,
			Preview:         a.Preview,
			Likes:           a.Likes,       // 直接 DB
			RepostCount:     a.RepostCount, // 直接 DB
			Commentcount:    a.CommentCount,
			CollectionCount: a.CollectionCount,
		})
	}
	// 写入缓存（仅首页）-明确给出缓存
	if useCache {
		if b, err := json.Marshal(items); err == nil {
			_ = global.RedisDB.Set(cacheKey, b, config.CacheTTL).Err()
		}
	}

	c.JSON(http.StatusOK, items)
}

// 个人文章管理列表响应项（比公开列表更详细）
type MyArticleItem struct {
	ID              uint   `json:"id"`
	Title           string `json:"title"`
	Preview         string `json:"preview"`
	Likes           uint   `json:"likes"`
	CollectionCount uint   `json:"collection_count"`
	CommentCount    uint   `json:"comment_count"`
	RepostCount     uint   `json:"repost_count"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// GetMyArticles godoc
// @Summary      获取当前用户的文章列表（管理用）
// @Tags         Articles
// @Security     Bearer
// @Produce      json
// @Param        page       query  int    false  "页码（默认1）"
// @Param        page_size  query  int    false  "每页条数（默认10，最大50）"
// @Param        order      query  string false  "排序：created_desc(默认)/created_asc/likes_desc/updated_desc"
// @Success      200        {array} controllers.MyArticleItem
// @Failure      401        {object} map[string]string
// @Failure      500        {object} map[string]string
// @Router       /articles/me [get]
func GetMyArticles(c *gin.Context) {
	userID := c.GetUint("user_id") // 从中间件获取

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	order := strings.TrimSpace(c.Query("order"))

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 50 { // 管理页一般不需要太大
		size = 50
	}

	db := global.DB.Model(&models.Article{}).Where("user_id = ?", userID)

	// 排序
	switch order {
	case "created_asc":
		db = db.Order("created_at ASC")
	case "likes_desc":
		db = db.Order("likes DESC")
	case "likes_asc":
		db = db.Order("likes ASC")
	case "comments_desc":
		db = db.Order("comment_count DESC")
	case "comments_asc":
		db = db.Order("comment_count ASC")
	case "reposts_desc":
		db = db.Order("repost_count DESC")
	case "reposts_asc":
		db = db.Order("repost_count ASC")
	case "collections_desc":
		db = db.Order("collection_count DESC")
	case "collections_asc":
		db = db.Order("collection_count ASC")
	default:
		db = db.Order("created_at DESC")
	}
	db = db.Select("id, user_id, title, preview, likes, repost_count, comment_count, collection_count, created_at, updated_at")
	var articles []models.Article
	if err := db.Preload("User", func(tx *gorm.DB) *gorm.DB {
		return tx.Select("id")
	}).Offset((page - 1) * size).Limit(size).Find(&articles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	items := make([]MyArticleItem, 0, len(articles))
	for _, a := range articles {
		var createdAt, updatedAt string
		if !a.CreatedAt.IsZero() {
			createdAt = a.CreatedAt.Format(utils.FormatTime_specific)
		}
		if !a.UpdatedAt.IsZero() {
			updatedAt = a.UpdatedAt.Format(utils.FormatTime_specific)
		}
		items = append(items, MyArticleItem{
			ID:              a.ID,
			Title:           a.Title,
			Preview:         a.Preview,
			Likes:           a.Likes,
			CollectionCount: a.CollectionCount,
			CommentCount:    a.CommentCount,
			RepostCount:     a.RepostCount,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		})
	}
	c.JSON(http.StatusOK, items)
}

// DeleteArticle godoc
// @Summary      永久删除当前用户的文章
// @Description  根据文章ID永久删除当前用户拥有的文章。该操作不可恢复。
// @Tags         Articles
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id   path  uint  true  "文章ID"
// @Success      200  {object}  map[string]string  "删除成功"
// @Failure      400  {object}  map[string]string  "文章ID无效"
// @Failure      401  {object}  map[string]string  "未认证"
// @Failure      403  {object}  map[string]string  "无权限访问" // 可选，但建议用 404 隐藏存在性
// @Failure      404  {object}  map[string]string  "文章不存在或无权限"
// @Failure      500  {object}  map[string]string  "服务器内部错误"
// @Router       /articles/{id} [delete]
func DeleteArticle(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid article id"})
		return
	}

	var article models.Article
	if err := global.DB.Where("id = ? AND user_id = ?", id, userID).First(&article).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database query failed"})
		return
	}

	err = global.DB.Transaction(func(tx *gorm.DB) error {
		articleID := uint(id)
		if err := tx.Where("article_id = ?", articleID).Delete(&models.Comment{}).Error; err != nil { //评论
			return err
		}
		if err := tx.Where("article_id = ?", articleID).Delete(&models.UserLikeArticle{}).Error; err != nil { //点赞关联表
			return err
		}
		if err := tx.Unscoped().Delete(&models.Article{}, articleID).Error; err != nil { //文章
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete article and related data"})
		return
	}

	// 3. 清理 Redis 缓存
	articleID := uint(id)
	global.RedisDB.Del(
		fmt.Sprintf(config.RedisLikeKey, articleID),
		fmt.Sprintf(config.RedisUserLikeKey, articleID, userID), // 点赞数缓存
		config.RedisHomePage,                                    //防止主页也出错
		config.RedisArticleKey,                                  //删除对应的文章存在的缓存
	)

	c.JSON(http.StatusOK, gin.H{"msg": "deleted"})
}

// 测试代码-业务运行时不用
func Get_ArticlesByID(c *gin.Context) {
	id := c.Param("id")
	var article models.Article
	if err := global.DB.Where("id = ?", id).First(&article); err != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error})
			return
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error})
			return
		}
	}
	c.JSON(200, article)
}
