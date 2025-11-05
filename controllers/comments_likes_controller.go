package controllers

import (
	"fmt"
	"net/http"
	"project/config"
	"project/global"
	"project/log"
	"project/models"
	"project/utils"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"

	"errors"

	"gorm.io/gorm"
)

//这里点赞包有redis的缓存

// ToggleLike godoc
// @Summary      点赞/取消点赞文章
// @Description  对指定文章进行点赞或取消点赞（切换）
// @Tags         Interactions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        article_id  path  uint  true  "文章ID"
// @Success      200  {object}  map[string]interface{}  {"like_flag":true,"total_likes":5}
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/articles/{article_id}/like [post]
func ToggleLike(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	articleID, err := strconv.ParseUint(c.Param("article_id"), 10, 32)
	if err != nil || articleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid article id"})
		return
	}

	//文章存在性检验
	IDkey := fmt.Sprintf(config.RedisArticleKey, articleID)
	cacheValue, err := global.RedisDB.Get(IDkey).Result()
	if err == nil {
		// 缓存命中
		if cacheValue == "0" {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		} // 缓存值为"1"，文章存在，继续执行后续逻辑
	} else if err == redis.Nil {
		// 缓存未命中，查询数据库
		var articleExists bool
		if err := global.DB.Model(&models.Article{}).Unscoped().
			Select("1").
			Where("id = ?", articleID).
			Scan(&articleExists).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		// 设置缓存值
		cacheValue = "0"
		if articleExists {
			cacheValue = "1"
		}
		global.RedisDB.Set(IDkey, cacheValue, config.Article_TTL)
		if !articleExists { //确实没有找到设置为0
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		}
	} else {
		// Redis错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache error"})
		return
	}

	likeKey := fmt.Sprintf(config.RedisLikeKey, articleID)
	userLikeKey := fmt.Sprintf(config.RedisUserLikeKey, articleID, userID)
	var likeFlag bool
	var newTotalLikes uint
	//  MySQL 事务：保证点赞关系 + 文章计数一致性
	err = global.DB.Transaction(func(tx *gorm.DB) error {
		var likeRecord models.UserLikeArticle
		err := tx.Where("user_id = ? AND article_id = ?", userID, articleID).First(&likeRecord).Error

		if errors.Is(err, gorm.ErrRecordNotFound) { // 点赞
			likeFlag = true
			if err := tx.Create(&models.UserLikeArticle{
				UserID:    userID,
				ArticleID: uint(articleID),
			}).Error; err != nil {
				return err
			}
		} else if err == nil { // 取消点赞
			likeFlag = false
			if err := tx.Delete(&likeRecord).Error; err != nil {
				return err
			}
		} else {
			return err
		}

		delta := map[bool]int{true: 1, false: -1}[likeFlag] //更新的+-
		if err := tx.Model(&models.Article{}).
			Where("id = ?", articleID).
			UpdateColumn("likes", gorm.Expr("likes + ?", delta)).Error; err != nil {
			return err
		}

		var article models.Article
		if err := tx.Select("likes").Where("id = ?", articleID).First(&article).Error; err != nil {
			return err
		}
		newTotalLikes = uint(article.Likes)
		return nil
	})
	//数据库事务报错
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
		}
		return
	}

	// 尽力更新 Redis（允许失败，不影响主流程）
	if likeFlag {
		global.RedisDB.Set(userLikeKey, "1", 7*24*time.Hour).Err() // 7天过期
		global.RedisDB.Set(likeKey, newTotalLikes, 7*24*time.Hour).Err()
	} else {
		global.RedisDB.Del(userLikeKey).Err()
		global.RedisDB.Set(likeKey, newTotalLikes, 7*24*time.Hour).Err()
	}

	// 返回结果（直接用 newTotalLikes，避免再查 Redis/DB）
	c.JSON(http.StatusOK, gin.H{
		"like_flag":   likeFlag,
		"total_likes": newTotalLikes,
	})
}

// 这里评论可以加入status进行评论-Todo
type commentResp struct {
	ID        uint   `json:"id"`
	Content   string `json:"content"`
	ParentID  *uint  `json:"parent_id"` // 改为 *uint
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}
type CommentCreateReq struct {
	ArticleID uint   `json:"article_id" binding:"required,min=1" example:"123"`
	ParentID  *uint  `json:"parent_id" example:"456"` // 为空表示一级评论
	Content   string `json:"content" binding:"required,max=1000" example:"写得真好！"`
}

// CreateComment 创建文章评论
//
// @Summary      创建评论
// @Description  用户对某篇文章发表评论，支持一级评论和回复（嵌套评论）
// @Tags         Comments
// @Accept       json
// @Produce      json
// @Param        body  body  CommentCreateReq  true  "评论内容"
// @Success      201   {object}  commentResp  "创建成功"
// @Failure      400   {object}  gin.H  "请求参数错误"
// @Failure      401   {object}  gin.H  "未授权"
// @Failure      404   {object}  gin.H  "文章不存在"
// @Failure      429   {object}  gin.H  "评论过于频繁"
// @Failure      500   {object}  gin.H  "服务器内部错误"
// @Router       /comments [post]
// @Security     ApiKeyAuth
func CreateComment(c *gin.Context) {
	userID := c.GetUint("user_id")
	userName := c.GetString("username")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CommentCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 🔒 防刷：限制用户评论频率（10秒/次）
	rateKey := fmt.Sprintf("comment:rate:user:%d", userID)
	if global.RedisDB.Exists(rateKey).Val() > 0 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请在10秒后再次评论"})
		return
	}
	global.RedisDB.Set(rateKey, "1", 10*time.Second)

	//文章存在性检验
	IDkey := fmt.Sprintf(config.RedisArticleKey, req.ArticleID)
	cacheValue, err := global.RedisDB.Get(IDkey).Result()
	if err == nil {
		// 缓存命中
		if cacheValue == "0" {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		} // 缓存值为"1"，文章存在，继续执行后续逻辑
	} else if err == redis.Nil {
		// 缓存未命中，查询数据库
		var articleExists bool
		if err := global.DB.Model(&models.Article{}).Unscoped().
			Select("1").
			Where("id = ?", req.ArticleID).
			Scan(&articleExists).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		// 设置缓存值
		cacheValue = "0"
		if articleExists {
			cacheValue = "1"
		}
		global.RedisDB.Set(IDkey, cacheValue, config.Article_TTL)
		if !articleExists { //确实没有找到设置为0
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		}
	} else {
		// Redis错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache error"})
		return
	}

	// 🔍 校验父评论（如果提供）
	if req.ParentID != nil {
		var parent models.Comment
		if err := global.DB.Select("id, article_id").
			Where("id = ? AND article_id = ?", *req.ParentID, req.ArticleID).
			First(&parent).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "parent comment not found or does not belong to this article"})
			return
		}
	}

	//在事务中创建评论并更新文章评论数
	var newComment models.Comment
	err = global.DB.Transaction(func(tx *gorm.DB) error {
		newComment = models.Comment{
			Content:   req.Content,
			UserID:    userID,
			ArticleID: req.ArticleID,
			ParentID:  req.ParentID,
		}
		if err := tx.Create(&newComment).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Article{}).
			Where("id = ?", req.ArticleID).
			UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create comment"})
		return
	}

	// 💬 构造响应
	resp := commentResp{
		ID:        newComment.ID,
		Content:   newComment.Content,
		ParentID:  newComment.ParentID, // *uint，nil 会转为 JSON null
		Username:  userName,
		CreatedAt: newComment.CreatedAt.Format(utils.FormatTime_specific),
	}

	c.JSON(http.StatusCreated, resp)
}

// 这里只有用户点击才能展开所有的评论情况
// 递归探寻用户的所有评论
type commentListResp struct {
	ID        uint              `json:"id"`
	Content   string            `json:"content"`
	ParentID  *uint             `json:"parent_id"` // null = 一级评论-实际上根据当前评论往下走
	Children  []commentListResp `json:"children"`
	Username  string            `json:"username"`
	CreatedAt string            `json:"created_at"`
}

// GetArticleComments 获取文章的所有评论（扁平列表）
//
// @Summary      获取文章评论列表
// @Description  返回某篇文章的所有评论（包括回复），按时间升序排列，前端可自行递归遍历多叉树结构
// @Tags         Comments
// @Produce      json
// @Param        id   path      uint  true  "文章ID"
// @Success      200  {array}   commentListResp
// @Failure      400  {object}  gin.H  "无效的文章ID"
// @Failure      404  {object}  gin.H  "文章不存在"
// @Failure      500  {object}  gin.H  "服务器错误"
// @Router       /articles/{id}/comments [get]
func GetArticleComments(c *gin.Context) { //依据文章id获取对应的所有评论
	articleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || articleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid article id"})
		return
	}
	//文章存在性检验
	IDkey := fmt.Sprintf(config.RedisArticleKey, articleID) //获取对应文章的ID检验
	cacheValue, err := global.RedisDB.Get(IDkey).Result()
	if err == nil {
		// 缓存命中但是值为0
		if cacheValue == "0" {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		} // 缓存值为"1"，文章存在，继续执行后续逻辑
	} else if err == redis.Nil {
		// 缓存未命中，查询数据库
		var articleExists bool
		if err := global.DB.Model(&models.Article{}).Unscoped().
			Select("1").
			Where("id = ?", articleID).
			Scan(&articleExists).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		// 设置缓存值
		cacheValue = "0"
		if articleExists {
			cacheValue = "1"
		}
		global.RedisDB.Set(IDkey, cacheValue, config.Article_TTL)
		if !articleExists { //确实没有找到设置为0
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		}
	} else {
		// Redis错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache error"})
		return
	}
	// 上述检验

	// 查询该文章的所有评论（一级 + 子评论），按时间排序 - 这里为了进行嵌套评论的构建-用多叉树的数据结构构建
	var comments []models.Comment
	if err := global.DB.
		Where("article_id = ?", articleID).
		Order("created_at ASC"). //按创建时间上述升序
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, username") // 预加载
		}).
		Find(&comments).Error; err != nil { //所有结果都放到切片里
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load comments"})
		return
	}

	resp := make([]commentListResp, len(comments)) //DTO操作只返回所需的数据
	for i, comment := range comments {
		username := "unknown" //初始化
		if comment.User != nil {
			username = comment.User.Username
		}
		resp[i] = commentListResp{
			ID:        comment.ID,                                          //对应的评论ID
			Content:   comment.Content,                                     //对应的内容
			ParentID:  comment.ParentID,                                    //父节点
			Username:  username,                                            //用户名
			CreatedAt: comment.CreatedAt.Format(utils.FormatTime_specific), //评论发表时间
		}
	}
	roots := buildCommentTree(resp)
	c.JSON(http.StatusOK, roots)
}

// 树的辅助函数
func buildCommentTree(comments []commentListResp) []commentListResp {
	commentsMap := make(map[uint]*commentListResp, len(comments))
	var roots []commentListResp
	for i := range comments {
		comment := &comments[i]
		comment.Children = []commentListResp{} //空接口的切片
		commentsMap[comment.ID] = comment      // 构建映射HASH表-因为接下来拿取用
	}
	for i := range comments {
		c := &comments[i] //要的是指针-这里是对原数据修改
		if c.ParentID != nil {
			if parent, ok := commentsMap[*c.ParentID]; ok { //防止出错的意外之险
				parent.Children = append(parent.Children, *c)
			} else {
				log.L().Error("comments structure error")
			}
		} else {
			roots = append(roots, *c) //接跟节点
		}
	}
	return roots
}
