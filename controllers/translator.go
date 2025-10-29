package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"project/log"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"project/config"
	"project/global"
	"project/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DTO格式
// 定义翻译请求结构体-前端所给的数据
type TranslationRequest struct {
	Text       string `json:"text" binding:"required"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang" binding:"required"`
	Model      string `json:"model"`
}

// 定义翻译响应结构体
type TranslationResponse struct {
	OriginalText   string `json:"original_text"`
	TranslatedText string `json:"translated_text"`
	SourceLang     string `json:"source_lang"`
	TargetLang     string `json:"target_lang"`
	Model          string `json:"model"`
}

var (
	// 只需要信号量控制并发
	translationSemaphore = make(chan struct{}, 100) // 限制最多100个并发请求
	historyLimitPerUser  = 50
)

// TranslateText 处理翻译请求并保存历史（同步保存，带短超时）-这里不加缓存因为翻译请求都是独特的文本
func TranslateText(c *gin.Context) {
	//并发限制-select 的规则是：只在“就绪”的分支里随机选一个
	select {
	case translationSemaphore <- struct{}{}: // 尝试想这个通道发送一个空的结构体
		defer func() { <-translationSemaphore }()
	case <-c.Request.Context().Done(): // 检测到前面请求或者响应的客户端断开
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "client canceled"})
		return
	case <-time.After(300 * time.Millisecond): // 等待300ms后仍未获取到名额（通道仍然满）
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "The internet server is busy, try later"})
		return
	}

	var req TranslationRequest // 接受前端的请求
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 预处理与校验-去除多余的空格
	req.SourceLang = strings.TrimSpace(req.SourceLang)
	req.TargetLang = strings.TrimSpace(req.TargetLang)
	req.Text = strings.TrimSpace(req.Text)

	// 默认值-如果没有设置自动判断
	if req.SourceLang == "" {
		req.SourceLang = "auto"
	}
	if req.Model == "" {
		req.Model = config.AppConfig.Translation_Api.Model
	}

	prompt := ""
	automode := (strings.ToLower(req.SourceLang) == "auto")
	// 构建翻译提示（尽量让模型只返回翻译文本）
	if automode { //确保只有小写
		prompt = fmt.Sprintf(`Please detect the source language of the text and translate it to %s.
Return ONLY a single JSON object and nothing else, in this exact shape:
{"detected_language":"<language_code>","translation":"<translated_text>"}
Do NOT add any commentary, labels, or extra text. Preserve original formatting (markdown, code blocks, newlines) inside "translation". Here is the text to translate:%s`,
			req.TargetLang, req.Text)
	} else {
		prompt = fmt.Sprintf(`Translate the following text from %s to %s.
Return ONLY a single JSON object and nothing else, in this exact shape:
{"translation":"<translated_text>"}
Do NOT add any commentary, labels, or extra text. Preserve original formatting (markdown, code blocks, newlines) inside "translation". Here is the text to translate:%s`,
			req.SourceLang, req.TargetLang, req.Text)
	}

	// 构建OpenAI风格的请求体-这里同一使用openai风格
	// 设置请求
	openaiReq := AIRequest{ //这里信息设置两片信息
		Model: req.Model,
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a professional translator. Translate the given text accurately while preserving the original meaning and tone.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
	}

	// marshal解析其响应
	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		log.L().Error("marshal openai request error: ", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal request"})
		return
	}

	// 发送请求到翻译 API
	base := c.Request.Context()
	ctx, cancel := context.WithTimeout(base, global.FetchTimeout*10) //这里请求设置久一点
	defer cancel()
	// 请求对象
	result, err := GetTranslatedText(c, ctx, req, reqBody, automode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Response error!"})
		return
	}

	// 获取当前用户ID（健壮处理多种类型）
	userID := c.GetUint("user_id")
	if userID > 0 { //
		// 同步保存，但绑定短超时，避免请求长时间阻塞
		db_ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		db := global.DB.WithContext(db_ctx)
		if err := SaveTranslationHistory(db, userID, req.Text, result.TranslatedText, req.SourceLang, req.TargetLang, req.Model, config.AppConfig.Translation_Api.Provider); err != nil {
			// 记录错误但不影响主流程
			log.L().Error("SaveTranslationHistory error: ", zap.Error(err))
		}
	}
	c.JSON(http.StatusOK, result)
}

// 这里是依据请求内容获得翻译内容

// GetSupportedLanguages 返回支持的语言列表
func GetSupportedLanguages(c *gin.Context) {
	languages := gin.H{
		"auto":  "Auto Detect",
		"en":    "English",
		"zh":    "Chinese (Simplified)",
		"zh-TW": "Chinese (Traditional)",
		"ja":    "Japanese",
		"ko":    "Korean",
		"es":    "Spanish",
		"fr":    "French",
		"de":    "German",
		"ru":    "Russian",
		"ar":    "Arabic",
		"pt":    "Portuguese",
		"it":    "Italian",
		"nl":    "Dutch",
		"sv":    "Swedish",
		"da":    "Danish",
		"no":    "Norwegian",
		"fi":    "Finnish",
		"pl":    "Polish",
		"tr":    "Turkish",
		"hi":    "Hindi",
		"th":    "Thai",
		"vi":    "Vietnamese",
	}
	c.JSON(http.StatusOK, languages)
}

// GetTranslationHistory 获取用户的翻译历史记录（分页）
func GetTranslationHistory(c *gin.Context) { //查询历史记录
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户无权限"})
		return
	}

	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	var histories []models.TranslationHistory
	var total int64

	// 先查询总数
	if err := global.DB.Model(&models.TranslationHistory{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		log.L().Error("count translation histories error:", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询翻译历史记录总数失败"})
		return
	}

	// 查询数据，兼容 timestamp 或 created_at
	if err := global.DB.Where("user_id = ?", userID).
		Order("created_at DESC, id DESC"). //按照搜索时间排序-降序
		Limit(pageSize).
		Offset(offset).
		Find(&histories).Error; err != nil {
		log.L().Error("The  Mysql database query translation histories error:", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询翻译历史记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"histories": histories,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// DeleteTranslationHistory 删除指定的历史记录
func DeleteTranslationHistory(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户无权限"})
		return
	}
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64) //解析为指定的类型
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的历史记录ID"})
		return
	}

	res := global.DB.Where("id = ? AND user_id = ?", uint(id), userID). //表示那一条记录id
										Delete(&models.TranslationHistory{})
	if res.Error != nil {
		log.L().Error("delete history error:", zap.Error(res.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除翻译历史记录失败"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "翻译历史记录不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "翻译历史记录已删除"})
}

// ClearTranslationHistory 清空用户的翻译历史记录
func ClearTranslationHistory(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户无权限"})
		return
	}
	//直接where再加Deltete删除对应用户即可
	if err := global.DB.Where("user_id = ?", userID).Delete(&models.TranslationHistory{}).Error; err != nil {
		log.L().Error("clear translation histories error:", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清空翻译历史记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "翻译历史记录已清空"})
}

// SaveTranslationHistory 保存翻译历史记录（可被单元测试替换）
func SaveTranslationHistory(db *gorm.DB, userID uint, src, dst, srcLang, tgtLang, model, provider string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1) 插入
		hist := models.TranslationHistory{
			UserID:         userID,
			SourceText:     src,
			TranslatedText: dst,
			SourceLang:     srcLang,
			TargetLang:     tgtLang,
			LLM:            model,
			Provider:       provider,
		}
		if err := tx.Create(&hist).Error; err != nil {
			return err
		}

		// 2) 找出“第 4 条起”的旧记录 (按时间逆序+ID 兜底，确保稳定排序)
		var oldIDs []uint
		if err := tx.Model(&models.TranslationHistory{}).
			Where("user_id = ?", userID).
			Order("created_at DESC, id DESC").       //降序-最新的排前面
			Offset(historyLimitPerUser).             // 跳过最新的 3 条 -队列优先级为最久弹出
			Pluck("id", &oldIDs).Error; err != nil { //查询并存入到切片里
			return err
		}

		// 3) 删除这些旧记录
		if len(oldIDs) > 0 {
			if err := tx.Where("id IN ?", oldIDs).
				Delete(&models.TranslationHistory{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
