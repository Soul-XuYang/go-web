package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"project/config"
	"project/log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ===========api结构体============= 这里采用OpenAPI格式
// 定义AI API请求结构体
type AIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// 定义AI API响应结构体
type AIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// 获取ai翻译文本-这里req是前端翻译文本的请求，req是响应体
func GetTranslatedText(c *gin.Context, ctx context.Context, req TranslationRequest, reqBody []byte, automode bool) (*TranslationResponse, error) {
	apiKey := config.AppConfig.Translation_Api.ApiKey
	reqURL := strings.TrimRight(config.AppConfig.Translation_Api.BaseURL, "/") + "/chat/completions" //先清除右'/'再+路径

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(reqBody)) //创建POST请求
	if err != nil {
		log.L().Error("create http request error :", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No apikey to create request"})
		return nil, fmt.Errorf("no api key provided") // 需要返回错误
	}

	client := &http.Client{Timeout: 20 * time.Second} //设置20s
	resp, err := client.Do(httpReq)
	if err != nil {
		log.L().Error("do translation request error::", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send request to translation service"})
		return nil, err
	}
	defer resp.Body.Close() //记得关闭响应体

	body, err := io.ReadAll(resp.Body) //用于从读取器读取所有数据
	if err != nil {
		log.L().Error("read translation response error::", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.L().Error("read translation response error")
		c.JSON(resp.StatusCode, gin.H{"error": "Translation service error", "details": string(body)})
		return nil, err
	}

	// 解析API的响应内容
	// 响应内容----------------------------
	var openaiResp AIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		log.L().Error("unmarshal translation response error:", zap.Error(err))
		return nil, err
	}

	if len(openaiResp.Choices) == 0 {
		log.L().Error("No translation choices in response")
		return nil, fmt.Errorf("no translation result returned")
	}

	content := strings.TrimSpace(openaiResp.Choices[0].Message.Content)

	var response struct {
		DetectedLanguage string `json:"detected_language,omitempty"`
		Translation      string `json:"translation"`
	}

	if err := json.Unmarshal([]byte(content), &response); err != nil { //这里是json格式的字符串-故而将结构体字符串解析为结构体
		log.L().Error("failed to parse translation response:", zap.Error(err))
		return nil, err
	}
	sourceLanguage := strings.TrimSpace(req.SourceLang)
	if automode {
		detected := strings.TrimSpace(response.DetectedLanguage)
		if detected != "" {
			sourceLanguage = detected
		}
	}
	if sourceLanguage == "" {
		sourceLanguage = "auto"
	}

	// 准备返回体
	result := &TranslationResponse{ //统一解析后获得的响应体
		OriginalText:   req.Text,
		TranslatedText: response.Translation,
		SourceLang:     sourceLanguage,
		TargetLang:     req.TargetLang,
		Model:          req.Model,
	}
	return result, nil
}
