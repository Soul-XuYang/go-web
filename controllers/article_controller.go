package controllers

import (
	"errors"
	"net/http"
	"project/global"
	"project/models"

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
	ID      uint   `json:"id"`
	Title   string `json:"title"`
	Preview string `json:"preview"`
	Likes   int    `json:"likes"`
	Created int64  `json:"created"`
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
// @Router       /api/create_articles [post]
func CreateArticle(c *gin.Context) {
	user_id := c.GetUint("user_id") // 中间件放进去的当前用户ID

	var in CreateArticleDTO //获取前端的数据                      // 创建DTO对象
	if err := c.ShouldBindJSON(&in); err != nil { //接受传来的数据对象
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将 DTO 映射到模型，服务端**显式**设置 UserID，避免前端伪造
	art := models.Article{
		UserID:  user_id,
		Title:   in.Title,
		Content: in.Content,
		Preview: in.Preview,
	}
	// 实际赋值还是找对应的赋值
	if err := global.DB.Create(&art).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 组织响应 DTO（不把敏感字段回给前端）
	resp := ArticleResp{
		ID: art.ID, Title: art.Title, Preview: art.Preview,
		Likes: art.Likes, Created: art.CreatedAt.Unix(),
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
// @Router       /api/update_articles/{id} [put]
// PUT/PATCH请求-更新文章
func UpdateArticle(c *gin.Context) {
	user_id := c.GetUint("user_id") // 中间件放进去的当前登录用户ID
	id := c.Param("id")

	var in UpdateArticleDTO //接受前端的数据
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 只更新非 nil 的字段
	updates := map[string]interface{}{}
	if in.Title != nil {
		updates["title"] = *in.Title
	}
	if in.Content != nil {
		updates["content"] = *in.Content
	}
	if in.Preview != nil {
		updates["preview"] = *in.Preview
	}

	// 只允许修改“我自己的那一行”
	tx := global.DB.Model(&models.Article{}).
		Where("id = ? AND user_id = ?", id, user_id).
		Updates(updates) // 会自动更新 UpdatedAt

	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tx.Error.Error()})
		return
	}
	if tx.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}

	// 返回更新后的数据（可选：再查一次）
	var out models.Article
	if err := global.DB.Where("id = ? AND user_id = ?", id, user_id).First(&out).Error; err != nil {
		// 正常不该失败；失败就返回 200+轻量确认
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusOK, out)
}

// 获取全部文章
// Get_All_Articles godoc
// @Summary      获取全部文章
// @Tags         Articles
// @Security     Bearer
// @Produce      json
// @Success      200  {array}   controllers.ArticleResp
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/articles [get]
func Get_All_Articles(c *gin.Context) {
	var all_articles []models.Article
	if err := global.DB.Find(&all_articles).Error; err != nil { //这里就是查询全部Find()，一般用where来进行条件查询
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]ArticleResp, 0, len(all_articles))
	for i := 0; i < len(all_articles); i++ {
		a := all_articles[i]
		out = append(out, ArticleResp{
			ID: a.ID, Title: a.Title, Preview: a.Preview,
			Likes: a.Likes, Created: a.CreatedAt.Unix(),
		})
	}
	c.JSON(http.StatusOK, out)
}

// 测试代码
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
