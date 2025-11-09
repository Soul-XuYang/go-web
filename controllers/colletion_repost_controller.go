package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"project/config"
	"project/global"
	"project/models"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
)

// 收藏夹名字的限制
const (
	maxCollectionNameLength = 50
	minCollectionNameLength = 1
)

func Repost(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission, user does not log in"})
		return
	}
	// 限制用户转发频率（3秒/次）
	rateKey := fmt.Sprintf(config.RedisRepostRate, userID)
	if global.RedisDB.Exists(rateKey).Val() > 0 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请在3秒后再次转发"})
		return
	}
	global.RedisDB.Set(rateKey, "1", 3*time.Second) //失效期

	//这里转发获取对应文章的ID
	articleID, err := strconv.ParseUint(c.Param("article_id"), 10, 32)
	if err != nil || articleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid article id"})
		return
	}
	// 这里先缓存查询文章的存在性，再通ID查询Mysql里是否有这个文章-带有缓存
	IDkey := fmt.Sprintf(config.RedisArticleKey, articleID)
	if cacheValue, err := global.RedisDB.Get(IDkey).Result(); err == nil {
		// 缓存命中
		if cacheValue == "0" {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		} // 缓存值为"1"，文章存在，继续执行后续逻辑
	} else if err == redis.Nil {
		// 缓存未命中，查询数据库
		var cnt int64
		if err := global.DB.Model(&models.Article{}).
			Where("id = ?", articleID).
			Count(&cnt).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if cnt == 0 {
			_ = global.RedisDB.Set(IDkey, "0", config.Article_TTL).Err()
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		}
		global.RedisDB.Set(IDkey, "1", config.Article_TTL) // 缓存文章存在性
	} else {
		// Redis错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache error"})
		return
	}

	// 查询是否已经转发过-缓存设置-这里就是缓存的妙用
	userFlagKey := fmt.Sprintf(config.RedisUserRepostKey, articleID, userID) // "article:repost:user:%d:%d"
	if flag, err := global.RedisDB.Get(userFlagKey).Result(); err == nil && flag == "1" {
		// 返回缓存里的总数（如果有）
		totalRepostKey := fmt.Sprintf(config.RedisRepostKey, articleID) // "article:repost:count:%d"
		totalStr, _ := global.RedisDB.Get(totalRepostKey).Result()
		totalReposts, _ := strconv.Atoi(totalStr)
		c.JSON(http.StatusOK, gin.H{
			"repost_flag":   true,
			"first_time":    false,
			"total_reposts": totalReposts,
		})
		return
	}

	var (
		inserted     bool
		totalReposts int64
	)
	err = global.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.UserArticleRepost{
				UserID:    userID,
				ArticleID: uint(articleID),
			})
		if res.Error != nil {
			return res.Error
		}
		inserted = (res.RowsAffected == 1)

		if inserted {
			// 只有首次才 +1
			if err := tx.Model(&models.Article{}).
				Where("id = ?", articleID).
				UpdateColumn("repost_count", gorm.Expr("repost_count + 1")).Error; err != nil {
				return err
			}
		}

		// 取最新计数
		if err := tx.Model(&models.Article{}).
			Where("id = ?", articleID).
			Pluck("repost_count", &totalReposts).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation failed"})
		return
	}

	// 这里回写缓存
	// 用户标记
	_ = global.RedisDB.Set(userFlagKey, "1", 24*time.Hour).Err() //设定已经进行第一次点赞了
	totalKey := fmt.Sprintf(config.RedisRepostKey, articleID)
	_ = global.RedisDB.Set(totalKey, strconv.FormatInt(totalReposts, 10), 24*time.Hour).Err() //设定文章点赞总数
	c.JSON(http.StatusOK, gin.H{
		"repost_flag":   true,         // 现在（或之前）已经转发过
		"first_time":    inserted,     // 这次是否首次
		"total_reposts": totalReposts, // 最新总数
	})
}

type createCollectionReq struct { //对应的文件夹的名字
	Name string `json:"name" binding:"required"`
}
type collectionResp struct {
	ID        uint   `json:"id" example:"123"`
	Name      string `json:"name" example:"我的收藏夹"`
	ItemCount uint   `json:"item_count" example:"0"`
}

// @Summary      create a collection
// @Description  给当前登录用户创建一个收藏夹（名称 1~50 字）
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      createCollectionReq  true  "收藏夹信息"
// @Success      200   {object}  collectionResp
// @Failure      400   {object}  gin.H
// @Failure      401   {object}  gin.H
// @Failure      500   {object}  gin.H
// @Router       /collections [post]
func CreateMycollection(c *gin.Context) { //创建个人的收藏夹
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}
	// 限制用户创建收藏夹频率（3秒/次）
	rateKey := fmt.Sprintf(config.RedisCreateCollectionRate, userID)
	if global.RedisDB.Exists(rateKey).Val() > 0 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请在3秒后再次创建收藏文件夹"})
		return
	}
	global.RedisDB.Set(rateKey, "1", 3*time.Second) //失效期


	var req createCollectionReq //接受发送来的请求-只有名字
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid params"})
		return
	}
	name := strings.TrimSpace(req.Name) // 去除无用的符号

	if name_length := len([]rune(name)); name_length < minCollectionNameLength || name_length > maxCollectionNameLength { //名字的长度验证-一定要用rune来保存验证
		c.JSON(http.StatusBadRequest, gin.H{"error": "name length must be 1~50"})
		return
	}
	collection := models.Collection{ //创建默认为0
		Name:      name,
		UserID:    userID,
		ItemCount: 0, // 默认为0
	}
	// 这里收藏夹可以同名的
	if err := global.DB.Create(&collection).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create collecion failed"})
		return
	}
	c.JSON(http.StatusOK, collectionResp{ //返回对应的数据即收藏夹数据很关键
		ID:        collection.ID,
		Name:      collection.Name,
		ItemCount: collection.ItemCount,
	})
}

// 因为这个要写成嵌套式响应-类似嵌套评论
type CollectionBriefResp struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	ItemCount uint   `json:"item_count"`
}

type MyCollectionsResp struct {
	Lists []CollectionBriefResp `json:"lists"`
	Sum   int                   `json:"sum"`
}

// @Summary      我的收藏夹（不含文章）
// @Description  返回当前登录用户的所有收藏夹（按创建倒序）
// @Tags         Collections
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  MyCollectionsResp
// @Failure      401  {object}  gin.H
// @Failure      500  {object}  gin.H
// GET /api/collections/all
func ListMyCollections(c *gin.Context) { //这个是用户选择加入哪个收藏夹时先给前端的响应
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}

	var cols []models.Collection
	if err := global.DB.Where("user_id = ?", userID).
		Order("id DESC").
		Find(&cols).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	resp := make([]CollectionBriefResp, 0, len(cols))
	for _, col := range cols {
		resp = append(resp, CollectionBriefResp{
			ID:        col.ID,
			Name:      col.Name,
			ItemCount: col.ItemCount,
		})
	}

	c.JSON(http.StatusOK, MyCollectionsResp{
		Lists: resp,
		Sum:   len(resp),
	})
}

// 二级嵌套
type CollectionItemResp struct { //单个文章item的数据
	ItemID     uint   `json:"item_id"`
	ArticleID  uint   `json:"article_id"`
	Title      string `json:"title"`
	AuthorID   uint   `json:"author_id"`
	AuthorName string `json:"author_name"`
	Preview    string `json:"preview"`
	CreatedAt  int64  `json:"created_at"`
}

type CollectionWithItemsResp struct { //同一文件夹下的所有item
	ID        uint                 `json:"id"`
	Name      string               `json:"name"`
	ItemCount uint                 `json:"item_count"`
	Items     []CollectionItemResp `json:"items"`
}

type MyCollectionsWithItemsResp struct { //用户对应的所有item
	Lists []CollectionWithItemsResp `json:"lists"`
	Sum   int                       `json:"sum"`
}

// @Summary      我的收藏夹（含各夹内全部文章）
// @Description  返回当前登录用户的所有收藏夹及各自包含的文章（按加入时间倒序）
// @Tags         Collections
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  MyCollectionsWithItemsResp  "OK"
// @Failure      401  {object}  gin.H
// @Failure      500  {object}  gin.H
// @Router       /api/collections/all_items [get]
// GET /api/collections/all_items
func ListMyCollectionsWithItems(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}

	// 这里先拉用户对应的收藏夹
	var cols []models.Collection
	if err := global.DB.Where("user_id = ?", userID).
		Order("id DESC").
		Find(&cols).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	if len(cols) == 0 { //返回全为空
		c.JSON(http.StatusOK, MyCollectionsWithItemsResp{
			Lists: []CollectionWithItemsResp{},
			Sum:   0,
		})
		return
	}

	// 构建映射表
	colIndex := make(map[uint]int, len(cols)) //收藏夹表
	resp := make([]CollectionWithItemsResp, len(cols))
	ids := make([]uint, 0, len(cols)) // 获得该用户的所有收藏表的ID
	for i, col := range cols {        //初始化最后的响应-这里cols为用户对应的收藏夹ID
		colIndex[col.ID] = i //收藏夹对应的序列数
		resp[i] = CollectionWithItemsResp{
			ID:        col.ID,
			Name:      col.Name,
			ItemCount: col.ItemCount,
			Items:     make([]CollectionItemResp, 0), //构建对应的文章表
		}
		ids = append(ids, col.ID)
	}

	// 一次性拉所有 items + 文章/作者信息，按加入时间倒序-对应CollectionWithItemsResp
	type row struct {
		CollectionID uint
		ItemID       uint
		ArticleID    uint
		Title        string
		AuthorID     uint
		AuthorName   string
		Preview      string
		CreatedAt    time.Time
	}

	var rows []row
	if err := global.DB.
		Table("collection_items AS ci"). //指定主表为item-更改列名加对应的数据写进去
		Select(`
			ci.collection_id,
			ci.id AS item_id, 
			ci.article_id,
			a.title,
			a.user_id AS author_id,
			u.username AS author_name,
			a.preview,
			ci.created_at AS created_at`).
		Joins(`JOIN articles AS a ON a.id = ci.article_id`).
		Joins(`LEFT JOIN users AS u ON u.id = a.user_id`).
		Where("ci.collection_id IN ?", ids). // 注意：GORM v2 用 IN ? 这里用文件夹的ID限制-ID对应rows的表
		Order("ci.id DESC").
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query items failed"})
		return
	}

	// 构建对应的item表
	for _, r := range rows { //如果 ids 切片中包含多个收藏夹ID，那么每个收藏夹ID都可能对应多个收藏项（row）
		idx := colIndex[r.CollectionID]                               //哈希表-获得对应的序列数
		resp[idx].Items = append(resp[idx].Items, CollectionItemResp{ //而这里是可以无限扩大的-指的是收藏夹对应的item扩大
			ItemID:     r.ItemID,
			ArticleID:  r.ArticleID,
			Title:      r.Title,
			AuthorID:   r.AuthorID,
			AuthorName: r.AuthorName,
			Preview:    r.Preview,
			CreatedAt:  r.CreatedAt.Unix(),
		})
	}

	c.JSON(http.StatusOK, MyCollectionsWithItemsResp{
		Lists: resp,
		Sum:   len(resp),
	})
}

// 添加到一个文章到我的收藏夹里
type addItemReq struct {
	CollectionID uint `json:"collection_id" binding:"required"`
	ArticleID    uint `json:"article_id" binding:"required"`
}

// @Summary     添加文章到我的收藏夹
// @Description 将指定文章加入到指定收藏夹；同一收藏夹不可重复加入同一文章
// @Tags        Collections
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       data body addItemReq true "收藏夹ID与文章ID"
// @Success     200 {object} gin.H "ok: true"
// @Failure     400 {object} gin.H
// @Failure     401 {object} gin.H
// @Failure     500 {object} gin.H
// @Router      /api/collections/item [post]
func AddArticleToMyCollection(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission,the user does not log in"})
		return
	}

	var req addItemReq //获得其请求的数据
	if err := c.ShouldBindJSON(&req); err != nil || req.CollectionID == 0 || req.ArticleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid params"})
		return
	}

	err := global.DB.Transaction(func(tx *gorm.DB) error { //事务操作
		// 校验收藏夹归属 & 加锁
		var coll models.Collection
		// 这里clause是添加SQL语句
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}). // 添加行级别的锁-锁定查到的行防止其他事务对齐修改
										Where("id = ? AND user_id = ?", req.CollectionID, userID).
										First(&coll).Error; err != nil { //获得第一手数据的收藏夹
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("collection not found or not owned by user")
			}
			return err
		}

		// 重复性检查-防止同一收藏夹重复收藏同一文章
		var exist models.CollectionItem
		if err := tx.Where("collection_id = ? AND article_id = ?", req.CollectionID, req.ArticleID).
			First(&exist).Error; err == nil { //查询是否存在
			return fmt.Errorf("article has already exists in the collection,cant add this article")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 如果不存在那就创建对应的CollectionItem
		item := models.CollectionItem{
			CollectionID: req.CollectionID,
			ArticleID:    req.ArticleID,
		}
		if err := tx.Create(&item).Error; err != nil { //创建item
			return err
		}

		//  接下来是易错点也是难点
		// 首先查询关联表这里要收藏加1-分为首次和首次之后
		var choosenItem models.UserCollectionItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}). //先加锁-防止修改
										Where("user_id = ? AND article_id = ?", userID, req.ArticleID).
										First(&choosenItem).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) { //找不到记录
				// 首次收藏
				item_connection := models.UserCollectionItem{ //创建收藏关联表
					UserID:    userID,
					ArticleID: req.ArticleID,
					ItemCount: 1,
				}
				if err := tx.Create(&item_connection).Error; err != nil {
					return err
				}
				if err := tx.Model(&models.Article{}).
					Where("id = ?", req.ArticleID).
					Update("collection_count", gorm.Expr("collection_count + 1")).Error; err != nil {
					return err
				}
			} else { //这个就是别的错误了-事务的写法
				return err
			}
		} else {
			// 之前已在别的收藏夹收藏过，计数 +1 这个其实是兜底操作-防止删除没有删除到总的收藏夹数
			if err := tx.Model(&models.UserCollectionItem{}).
				Where("user_id = ? AND article_id = ?", userID, req.ArticleID).
				Update("item_count", gorm.Expr("item_count + 1")).Error; err != nil {
				return err
			}
		}

		// 对应的收藏夹条目数 +1
		if err := tx.Model(&models.Collection{}).
			Where("id = ?", req.CollectionID).
			Update("item_count", gorm.Expr("item_count + 1")).Error; err != nil {
			return err
		}

		return nil
	})
	// 针对事务的操作
	if err != nil { //错误分开说
		msg := err.Error()
		// 针对于字符串的语句
		if strings.Contains(msg, "not found") ||
			strings.Contains(msg, "not owned") ||
			strings.Contains(msg, "already exists") {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type removeItemResp struct {
	Ok        bool `json:"ok" example:"true"`
	ItemCount uint `json:"item_count" example:"4"`
}
type removeItemReq struct {
	CollectionID uint `json:"collection_id" binding:"required"`
	ArticleID    uint `json:"article_id" binding:"required"`
}

// @Summary     从收藏夹移除文章
// @Description 从指定收藏夹里删除指定文章；若该用户对该文的总收藏数从 1->0，则文章的 collection_count -1|注意:对应的两个参数都在请求里
// @Tags        Collections
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       data body removeItemReq true "收藏夹ID与文章ID"
// @Success     200 {object} removeItemResp "OK"
// @Failure     400 {object} gin.H
// @Failure     401 {object} gin.H
// @Failure     500 {object} gin.H
// @Router      /api/collections/item [delete]
func RemoveArticleFromMyCollection(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission, user does not log in"})
		return
	}
	var req removeItemReq
	if err := c.ShouldBindJSON(&req); err != nil || req.CollectionID == 0 || req.ArticleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid params"})
		return
	}
	collectionID := req.CollectionID
	articleID := req.ArticleID

	var itemCount uint //收藏夹的总个数

	err := global.DB.Transaction(func(tx *gorm.DB) error {
		// 检验文件夹是否存在
		if err := tx.Select("id").Where("id = ? AND user_id = ?", collectionID, userID).First(&models.Collection{}).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("collection has not found or not owned by user")
			}
			return err
		}

		// 这里先查找并删除对应的item
		res := tx.Where("collection_id = ? AND article_id = ?", collectionID, articleID).Delete(&models.CollectionItem{})
		if res.Error != nil {
			return res.Error //错误
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("article not in the collection") //找不到
		}
		// 维护收藏夹和文章的收藏数
		// 这里是更新收藏夹的个数
		if err := tx.Model(&models.Collection{}).
			Where("id = ?", collectionID).
			UpdateColumn("item_count", gorm.Expr("CASE WHEN item_count > 0 THEN item_count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
		// 下列是item关联表
		var item_connection models.UserCollectionItem
		if err := tx.Where("user_id = ? AND article_id = ?", userID, articleID).First(&item_connection).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("collection item not found")
			}
			return err
		}

		if item_connection.ItemCount <= 1 { //如果小于或则等于1这里我们就直接删除了
			if err := tx.Where("user_id = ? AND article_id = ?", userID, articleID).Delete(&models.UserCollectionItem{}).Error; err != nil {
				return err
			} //并且对应的文章的收藏数-1
			if err := tx.Model(&models.Article{}).
				Where("id = ?", articleID).
				UpdateColumn("collection_count", gorm.Expr("CASE WHEN collection_count > 0 THEN collection_count - 1 ELSE 0 END")).Error; err != nil {
				return err
			}
		} else { //只对关联表操作
			if err := tx.Model(&models.UserCollectionItem{}).
				Where("user_id = ? AND article_id = ?", userID, articleID).
				UpdateColumn("item_count", gorm.Expr("item_count - 1")).Error; err != nil {
				return err
			}
		}
		// 读取最新总数（只取该列）
		if err := tx.Model(&models.Collection{}).
			Where("id = ?", collectionID).
			Pluck("item_count", &itemCount).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"),
			strings.Contains(msg, "not owned"),
			strings.Contains(msg, "not in the collection"):
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		}
		return
	}

	c.JSON(http.StatusOK, removeItemResp{
		Ok:        true,
		ItemCount: itemCount,
	})
}

// @Summary     删除用户指定的收藏夹
// @Description 删除指定收藏夹；会逐条移除该夹内的文章，并正确维护计数后再删除收藏夹本身-内部的文章会全部删除
// @Tags        Collections
// @Security    BearerAuth
// @Produce     json
// @Param       id   path     int  true "收藏夹ID"
// @Success     200  {object} deleteResp "ok: true"
// @Failure     400  {object} gin.H
// @Failure     401  {object} gin.H
// @Failure     500  {object} gin.H
// @Router      /api/collections/{id} [delete]
func DeleteMyCollection(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission, user does not log in"})
		return
	}

	collectionId, err := strconv.ParseUint(c.Param("collectionId"), 10, 64) //获取对应的收藏夹ID
	if err != nil || collectionId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection id"})
		return
	}
	collectionID := uint(collectionId)

	err = global.DB.Transaction(func(tx *gorm.DB) error {
		var collection models.Collection // 这里查询对应的收藏夹-以用户和文件夹的id
		// 锁定收藏夹行（防并发新增/删除）
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", collectionID, userID).
			First(&collection).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("collection not found or not owned by user")
			}
			return err
		}
		//上述为查找文件夹

		// 接受的数据-这里不需要展示这么多-直接按照ID删除就行
		var items []struct {
			ID        uint
			ArticleID uint
		}
		// var items []models.CollectionItem - 待思考的数据
		if err := tx.Model(&models.CollectionItem{}).
			Select("id, article_id").
			Where("collection_id = ?", collectionID).
			Order("id DESC").
			Scan(&items).Error; err != nil {
			return err
		}

		if len(items) > 0 { //说明有文章item
			if err := tx.Where("collection_id = ?", collectionID).Delete(&models.CollectionItem{}).Error; err != nil { //删除对应的item
				return err
			}
			for _, item := range items {
				var uci models.UserCollectionItem //这里是关联表-获取对应的关联表
				if err := tx.Where("user_id = ? AND article_id = ?", userID, item.ArticleID).First(&uci).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return err
				}
				// 如果其本身等于1-直接删表-同时对应的文章收藏数-1
				if uci.ItemCount <= 1 {
					// 删除关联表
					if err := tx.Where("user_id = ? AND article_id = ?", userID, item.ArticleID).Delete(&models.UserCollectionItem{}).Error; err != nil {
						return err
					}
					// 删除对应的收藏数
					if err := tx.Model(&models.Article{}).
						Where("id = ?", item.ArticleID).
						UpdateColumn("collection_count", gorm.Expr("CASE WHEN collection_count > 0 THEN collection_count - 1 ELSE 0 END")).Error; err != nil {
						return err
					}
				} else { //对应的关联表数-1
					if err := tx.Model(&models.UserCollectionItem{}).
						Where("user_id = ? AND article_id = ?", userID, item.ArticleID).
						UpdateColumn("item_count", gorm.Expr("item_count - 1")).Error; err != nil {
						return err
					}
				}
			}
		}

		if err := tx.Where("id = ?", collectionID).Delete(&models.Collection{}).Error; err != nil { //最后删除对应的收藏夹
			return err
		}

		return nil
	})
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"),
			strings.Contains(msg, "not owned"):
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		}
		return
	}
	c.JSON(http.StatusOK, &deleteResp{Ok: true})
}

type deleteResp struct {
	Ok bool `json:"ok" example:"true"`
}
