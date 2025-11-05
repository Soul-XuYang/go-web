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

func Repost(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission, user does not log in"})
		return
	}
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

	// ---------- 5) 返回 ----------
	c.JSON(http.StatusOK, gin.H{
		"repost_flag":   true,         // 现在（或之前）已经转发过
		"first_time":    inserted,     // 这次是否首次
		"total_reposts": totalReposts, // 最新总数
	})
}

type createCollectionReq struct {
	Name string `json:"name" binding:"required"`
}
type collectionResp struct {
	ID        uint   `json:"id" example:"123"`
	Name      string `json:"name" example:"我的收藏夹"`
	ItemCount uint   `json:"item_count" example:"0"`
}

// @Summary      create a collection
// @Description  给当前登录用户创建一个收藏夹（名称 1~100 字）
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission, user does not log in"})
		return
	}

	var req createCollectionReq //接受发送来的请求
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid params"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name_length := len([]rune(name)); name_length == 0 || name_length > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name length must be 1~100"})
		return
	}
	collection := models.Collection{
		Name:      name,
		UserID:    userID,
		ItemCount: 0, // 默认为0
	}
	if err := global.DB.Create(&collection).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create collecion failed"})
		return
	}
	c.JSON(http.StatusOK, collectionResp{
		ID:        collection.ID,
		Name:      collection.Name,
		ItemCount: collection.ItemCount,
	})
}

type addItemReq struct {
	ArticleID uint `json:"article_id" binding:"required"`
}

type addItemResp struct {
	Ok            bool `json:"ok" example:"true"`
	ItemCount     uint `json:"item_count" example:"5"`        // 夹内数量
	CollectionCnt uint `json:"collection_count" example:"12"` // 该文章被收藏总次数（可选返回）
}

// @Summary      add an article to a collection
// @Description  将一篇文章加入指定收藏夹（同一文章在同一收藏夹内仅出现一次）
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        collectionId    path      int          true  "收藏夹ID"
// @Param        body  body      addItemReq   true  "要收藏的文章"
// @Success      200   {object}  addItemResp
// @Failure      400   {object}  gin.H
// @Failure      401   {object}  gin.H
// @Failure      404   {object}  gin.H
// @Failure      500   {object}  gin.H
// @Router       /collections/{collectionId}/items [post]
func AddArticleToMyCollection(c *gin.Context) { //一是指定资源二是指定文章--这里映射是对应的文件夹数+1
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission, user does not log in"})
		return
	}

	collectionId, err := strconv.ParseUint(c.Param("collectionId"), 10, 64)
	if err != nil || collectionId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection id"})
		return
	}
	collectionID := uint(collectionId)
	var req addItemReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ArticleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid params"})
		return
	}

	var (
		ItemCount          uint //文件夹的个数
		totalCollectionCnt uint //文章的收藏个数
		articleExist       bool
	)
	IDKey := fmt.Sprintf(config.RedisArticleKey, req.ArticleID)
	if val, err := global.RedisDB.Get(IDKey).Result(); err == nil {
		if val == "0" {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		}
		articleExist = true
	}
	// 收藏夹存在性检查
	if err := global.DB.Transaction(func(tx *gorm.DB) error {
		// 1) 校验收藏夹归属
		var col models.Collection
		if err := tx.Select("id,user_id,item_count").
			Where("id=? AND user_id=?", collectionID, userID).
			First(&col).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
				return err
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return err
		}
		// 校验文章是否存在
		if !articleExist {
			var cnt int64
			if err := global.DB.Model(&models.Article{}).Where("id = ?", req.ArticleID).Count(&cnt).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return err
			}
			if cnt == 0 {
				_ = global.RedisDB.Set(IDKey, "0", config.Article_TTL).Err()
				c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
				return err
			}
			_ = global.RedisDB.Set(IDKey, "1", config.Article_TTL).Err()
			//这个之后就是文章存在了
		}
		// 使用从句创建索引-高并发下更小-这里查的是关系表
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.CollectionItem{
				CollectionID: collectionID,
				ArticleID:    req.ArticleID,
			}) //res 是 GORM 的 *gorm.DB 类型的结果对象
		if res.Error != nil {
			return res.Error
		}
        //注意这里是对应的变动关系表的行数-如果变动则是第一个创建-即第一次插入
		if res.RowsAffected == 1 {
			if err := tx.Model(&models.Collection{}).
				Where("id=?", collectionID).
				UpdateColumn("item_count", gorm.Expr("item_count + 1")).Error; err != nil {
				return err
			}
		}
		// 读取最新计数用于返回
		if err := tx.Model(&models.Collection{}). //当前文件夹的个数
								Where("id=?", collectionID).
								Pluck("item_count", &ItemCount).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Article{}). //当前文章的收藏数
							Where("id=?", req.ArticleID).
							Pluck("collection_count", &totalCollectionCnt).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		// 统一错误返回（可细分唯一键冲突 -> 200 幂等成功）
		c.JSON(http.StatusBadRequest, gin.H{"error": "add failed"})
		return
	}
	c.JSON(http.StatusOK, addItemResp{
		Ok:            true,
		ItemCount:     ItemCount,
		CollectionCnt: totalCollectionCnt,
	})
}

// -------------------------------------------
type ArticleBriefResp struct {
	ID           uint   `json:"id"`
	Title        string `json:"title"`
	Preview      string `json:"preview"`
	Likes        uint   `json:"likes"`
	RepostCount  uint   `json:"repost_count"`
	CommentCount uint   `json:"comment_count"`
}

// 因为这个要写成嵌套式响应-类似嵌套评论
type CollectionWithItemsResp struct {
	ID        uint               `json:"id"`
	Name      string             `json:"name"`
	ItemCount uint               `json:"item_count"`
	Items     []ArticleBriefResp `json:"items"`
}

// @Summary      我的收藏夹（含各夹内全部文章）
// @Description  返回当前登录用户的所有收藏夹及各自包含的文章（文章按照加入收藏的时间倒序）
// @Tags         Collections
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   CollectionWithItemsResp
// @Failure      401  {object}  gin.H
// @Failure      500  {object}  gin.H
// GET /api/collections/all_items
func ListmyCollection(c *gin.Context) { //列出当前用的收藏文件夹和对应文件夹收藏的文件
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no permission"})
		return
	}
	// 1) 所有收藏夹
	var lists []models.Collection
	if err := global.DB.Model(&models.Collection{}). //遍历所有收藏夹表-获取对应的id、name和个数
								Select("id, name, item_count").
								Where("user_id = ?", userID). //where限制并且按照日期降序
								Order("created_at DESC").
								Find(&lists).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "collections query failed"})
		return
	}
	if len(lists) == 0 {
		c.JSON(http.StatusOK, []CollectionWithItemsResp{}) //返回空接口
		return
	}
	//按照那里的收藏夹文件id找对应的文件-获取对应的文件夹ID
	IDMaps := make([]uint, 0, len(lists))
	for _, v := range lists {
		IDMaps = append(IDMaps, v.ID)
	}

	//  一次性把所有条目拉出来（JOIN 文章；按加入时间倒序）
	type article struct {
		CollectionID uint
		ID           uint
		Title        string
		Preview      string
		Likes        uint
		RepostCount  uint
		CommentCount uint
	}
	var list []article
	if err := global.DB.Table("collection_items AS ci"). //别名
								Select("ci.collection_id, a.id, a.title, a.preview, a.likes, a.repost_count, a.comment_count"). //查询文章内容和收藏夹名
								Joins("JOIN articles a ON a.id = ci.article_id").                                               //别名a 内连接 articles
								Where("ci.collection_id IN (?)", IDMaps).                                                       //对应的文件夹id
								Order("ci.collection_id, ci.created_at DESC").                                                  //第一个分块，第二个按时间倒序
								Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "items query failed"})
		return
	} //获取的数据是文章内容+收藏夹ID，而匹配条件为哈希表中的

	// 内部的映射
	bucket := make(map[uint][]ArticleBriefResp, len(lists)) //这里是对应收藏文件夹的id->多个文章的id映射
	for _, r := range list {
		bucket[r.CollectionID] = append(bucket[r.CollectionID], ArticleBriefResp{
			ID:           r.ID,
			Title:        r.Title,
			Preview:      r.Preview,
			Likes:        r.Likes,
			RepostCount:  r.RepostCount,
			CommentCount: r.CommentCount,
		})
	}

	out := make([]CollectionWithItemsResp, 0, len(lists))
	for _, c0 := range lists {
		out = append(out, CollectionWithItemsResp{
			ID:        c0.ID,
			Name:      c0.Name,
			ItemCount: c0.ItemCount,
			Items:     bucket[c0.ID], //对应的映射数据
		})
	}

	c.JSON(http.StatusOK, out)
}

//前端收藏数，因为它+1后续再考虑-因为这个是一个牵一发而动全身的事情
