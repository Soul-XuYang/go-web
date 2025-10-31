package controllers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"project/config"
	"project/global"
	"project/models"
	"project/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 上传的文件的格式要求
var (
	allowedExts = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
		".txt": true, ".md": true, ".csv": true, ".pdf": true, ".docx": true,
		".doc": true, ".xlsx": true,
	}
)

type UploadResponse struct {
	Msg  string `json:"msg"`
	ID   uint   `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
	Size string `json:"size"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// UploadFile godoc
// @Summary      上传文件
// @Description  使用相对路径存储；服务端校验扩展名/MIME、配额，写入相对目录（基于 CWD 的 config.AppConfig.Upload.Storagepath），并保存元信息到数据库。
// @Tags         Files
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file     formData  file   true  "文件（必填）"
// @Param        content  formData  string false "文件描述/备注"
// @Success      200  {object}  UploadResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /files/upload [post]
func UploadFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	baseRel := config.AppConfig.Upload.Storagepath                   // 这里是（相对）路径
	maxLoad := int64(config.AppConfig.Upload.FileSize) * 1024 * 1024 //都化为64
	maxTotal := int64(config.AppConfig.Upload.TotalSize) * 1024 * 1024

	// 取文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file or invalid form"})
		return
	}
	defer file.Close()

	// 大小/扩展名
	if header.Size > maxLoad {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("This file is too large (max %sMB)", (fmt.Sprintf("%.2f", float64(maxLoad/1024/1024))))})
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file type not allowed"})
		return
	}

	// MIME 嗅探
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(file, sniff)
	if n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This is an empty file,can't upload"})
		return
	}
	contentType := http.DetectContentType(sniff[:n]) //使用
	reader := io.MultiReader(bytes.NewReader(sniff[:n]), file)

	// 配额（写盘前）判断大小
	var totalSize int64
	global.DB.Model(&models.Files{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&totalSize)
	if totalSize+header.Size > maxTotal {
		c.JSON(http.StatusBadRequest, gin.H{"error": "storage limit exceeded"})
		return
	}

	// 目录 & 相对 key
	dateDir := time.Now().Format("2006-01-02")
	relDir := filepath.Join(fmt.Sprintf("user_%d", userID), dateDir)    //用户在文件里保存的路径名字
	if dirPath, err := utils.SafeJoinRel(baseRel, relDir); err != nil { //这里dirPath是完整的相对路径
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	} else if err := os.MkdirAll(dirPath, 0755); err != nil { //创建
		c.JSON(http.StatusInternalServerError, gin.H{"error": "The syesytem create dir failed"})
		return
	}

	baseName := filepath.Base(header.Filename)                                           // 清洗并获得其文件名+拓展名
	tmpRel, _ := utils.SafeJoinRel(baseRel, filepath.Join(relDir, "."+baseName+".part")) //先创建临时文件
	finalRel, _ := utils.SafeJoinRel(baseRel, filepath.Join(relDir, baseName))           //完整的最终路径

	out, err := os.Create(tmpRel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create temp file failed"})
		return
	}
	defer out.Close()

	fileHash, written, err := utils.CopyWithHash(out, reader, maxLoad, header.Size) //边写入文件边读取其Hash值
	if err != nil {                                                                 //依据结果写错误情况
		_ = os.Remove(tmpRel)
		switch {
		case errors.Is(err, utils.ErrSizeExceeded):
			c.JSON(http.StatusBadRequest, gin.H{"error": "file size exceeded limit"})
		case errors.Is(err, utils.ErrSizeMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"error": "file size mismatch or exceeded"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write file failed"})
		}
		return
	}

	// 去重（按内容+用户）
	var cnt int64
	global.DB.Model(&models.Files{}).
		Where("hash = ? AND user_id = ?", fileHash, userID).
		Count(&cnt)
	if cnt > 0 {
		_ = os.Remove(tmpRel)
		c.JSON(http.StatusOK, &UploadResponse{
			Msg:  "文件内容未变化，已存在相同文件。",
			Size: utils.Get_size(written),
		})
		return
	}

	if _, statErr := os.Stat(finalRel); statErr == nil {
		_ = os.Remove(finalRel)
	}
	if err := os.Rename(tmpRel, finalRel); err != nil {
		_ = os.Remove(tmpRel)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "The system commit file failed"})
		return
	}

	// 相对 key（存库/对外） - 这个易错保证所有系统上的路径分割符一致 - 易错这个
	finalKey := filepath.ToSlash(filepath.Join(relDir, baseName)) //不包括主的相对路径

	// 入库
	newFile := models.Files{
		UserID:   userID,
		Filename: baseName,
		FileType: contentType,
		FilePath: finalKey, // 相对 key
		FileSize: written,
		Hash:     fileHash,
		FileInfo: c.PostForm("content"), //上传的的文本信息内容
	}
	if err := global.DB.Create(&newFile).Error; err != nil {
		_ = os.Remove(finalRel)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save to database failed"})
		return
	}

	c.JSON(http.StatusOK, &UploadResponse{
		Msg:  "该文件上传成功！",
		ID:   newFile.ID,
		URL:  fmt.Sprintf("/files/%d", newFile.ID),
		Size: utils.Get_size(written), //B
	})
}

// DownloadFile godoc
// @Summary      下载/预览文件
// @Description  根据文件ID下载或预览；支持 Range/304。query: download=1 为附件下载，否则 inline 预览。服务端会在成功响应时为该文件的下载次数 +1，并通过响应头 `X-Download-Count` 回传最新次数。
// @Tags         Files
// @Produce      application/octet-stream
// @Security     BearerAuth
// @Param        id        path   int    true  "文件ID"
// @Param        download  query  int    false "1=attachment; 省略或0=inline"
// @Success      200       {file}  file  "响应头包含 X-Download-Count、ETag、Last-Modified 等"
// @Header       200       {string}  X-Download-Count  "最新下载次数"
// @Failure      401       {object}  ErrorResponse
// @Failure      403       {object}  ErrorResponse
// @Failure      404       {object}  ErrorResponse
// @Failure      500       {object}  ErrorResponse
// @Router       /files/{id} [get]
func DownloadFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64) //选用指定的文件

	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var f models.Files
	if err := global.DB.First(&f, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if f.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission ,forbidden"})
		return
	}

	baseRel := config.AppConfig.Upload.Storagepath         // 相对基底，例如 "files"
	relPath, err := utils.SafeJoinRel(baseRel, f.FilePath) //这里是从数据库中取相对base的文件路径-安全拼接加以保存

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "It's invalid stored path"})
		return
	}
	fp, err := os.Open(relPath) //路径存在
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "This file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "The system opens file failed"})
		}
		return
	}
	defer fp.Close() //易忘

	stat, err := fp.Stat()          //获取当前文件状态
	if err != nil || stat.IsDir() { //看其是否为目录
		c.JSON(http.StatusNotFound, gin.H{"error": "This file not found"})
		return
	}
	modTime := stat.ModTime() //获取文件的最后修改时间

	if err := global.DB.
		Model(&models.Files{}). //更新
		Where("id = ? AND user_id = ?", f.ID, userID).
		UpdateColumn("downloads", gorm.Expr("downloads + ?", 1)).Error; err == nil { //gorm.Expr创建一个SQL表达式-对这里的参数+1

		//获取其值
		var newCnt int64
		_ = global.DB.Model(&models.Files{}).
			Where("id = ?", f.ID).
			Select("downloads").
			Scan(&newCnt).Error
		c.Header("X-Download-Count", strconv.FormatInt(newCnt, 10)) //只要找到文件就下载量+1
	}

	etag := `W/"` + f.Hash + `"` // W/+"文件内容的哈希值"
	// 获取客户端即前端传来的ETag验证，获取其请求头并使用strings.Contains查找是否有对应的字串部分
	if inm := c.GetHeader("If-None-Match"); inm != "" && strings.Contains(inm, f.Hash) {
		c.Header("ETag", etag)
		c.Status(http.StatusNotModified) //提前结束处理不发送文件内容
		return
	}
	// 这里是缓存验证机制
	if ims := c.GetHeader("If-Modified-Since"); ims != "" { // 获取请求头的If-Modified-Since值
		if t, perr := http.ParseTime(ims); perr == nil && !modTime.After(t) { //解析客户端发送的时间字符串-检查客户端缓存的时间是否晚于文件修改时间
			// 响应头都是键值对形式
			c.Header("ETag", etag)                                           //设置对应的ETag头和Last-Modified
			c.Header("Last-Modified", modTime.UTC().Format(http.TimeFormat)) //将文件上次修改的时间传回去
			c.Status(http.StatusNotModified)
			return
		}
	}

	ct := f.FileType //获取文件的类型
	if ct == "" {
		ct = "application/octet-stream" //默认二进制流类型
	}
	c.Header("Content-Type", ct)
	c.Header("ETag", etag)
	c.Header("Last-Modified", modTime.UTC().Format(http.TimeFormat))

	disp := "inline"                //浏览器会尝试直接显示文件
	if c.Query("download") == "1" { //切换为强制下载
		disp = "attachment"
	}
	filename := filepath.Base(f.Filename)                                                                   //去除路径
	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename*=UTF-8''%s`, disp, url.PathEscape(filename))) //UTF8处理文件名-URL文件名编码，到时候直接访问这个

	http.ServeContent(c.Writer, c.Request, filename, modTime, fp) //这个是文件流响应
}

// 依旧是给出文件id然后删除
// DeleteFile godoc
// @Summary      删除文件
// @Description  根据ID删除当前用户的文件（先删磁盘，再删数据库）；文件不存在将被忽略以便幂等。
// @Tags         Files
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  int  true  "文件ID"
// @Success      200  {object}  map[string]string
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /files/{id} [delete]
func DeleteFile(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}
	// 获得用户id和文件id-------------------------------

	var f models.Files // 先查找后端数据库中的文件id
	if err := global.DB.First(&f, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	if f.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "useless acount,forbidden"})
		return
	}

	baseRel := config.AppConfig.Upload.Storagepath           // 相对基底，例如 "files"
	finalPath, err := utils.SafeJoinRel(baseRel, f.FilePath) //获取文件的路径
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stored path"})
		return
	}

	// 事务：确保“删文件失败”能回滚数据库
	if err := global.DB.Transaction(func(tx *gorm.DB) error {
		// 先删磁盘（不存在则忽略，保证幂等）
		if err := os.Remove(finalPath); err != nil && !os.IsNotExist(err) { //os.IsNotExist(err)如果其它类型错误存在false
			return fmt.Errorf("remove file failed: %w", err)
		}
		// 删数据库
		if err := tx.Delete(&models.Files{}, f.ID).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "deleted"})
}

// DTO结构
type FileItem struct {
	ID          uint      `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Hash        string    `json:"hash"`
	Path        string    `json:"path"` // 相对 key（不暴露真实磁盘根）
	CreatedAt   time.Time `json:"created_at"`
}

type ListFilesResponse struct {
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Items    []FileItem `json:"items"`
}

// ListMyFiles godoc
// @Summary      列出当前用户的文件
// @Description  支持按关键字、扩展名、MIME、时间范围、大小范围筛选，分页返回；按 created_at 排序。
// @Tags         Files
// @Produce      json
// @Security     BearerAuth
// @Param        q            query  string false "关键字（匹配文件名，模糊）"
// @Param        ext          query  string false "扩展名（如 .pdf/.png，不区分大小写）"
// @Param        content_type query  string false "MIME 前缀（如 image/、application/pdf）"
// @Param        date_from    query  string false "起始日期（YYYY-MM-DD）"
// @Param        date_to      query  string false "结束日期（YYYY-MM-DD，含当日）"
// @Param        min_size     query  int    false "最小大小（字节）"
// @Param        max_size     query  int    false "最大大小（字节）"
// @Param        page         query  int    false "页码（默认1）"
// @Param        page_size    query  int    false "每页的条数（默认20，最大100）"
// @Param        order        query  string false "排序：共四种组合，两种排序方式-上传日期和文件大小 created_desc（默认）/created_asc/size_desc/size_asc"
// @Success      200  {object}  ListFilesResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /files/lists [get]
func ListMyFiles(c *gin.Context) { //展示用户的所有文件
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 解析HTTP请求，这个查询可以以各种各样的参数进行查询搜索
	//查询q 后缀ext 文件文本类型 起始日期 结束日子 最小文件大小 最大文件
	q := strings.TrimSpace(c.Query("q"))
	ext := strings.ToLower(strings.TrimSpace(c.Query("ext"))) //查询后缀即拓展名
	ctPrefix := strings.TrimSpace(c.Query("content_type"))
	dateFrom := strings.TrimSpace(c.Query("date_from"))
	dateTo := strings.TrimSpace(c.Query("date_to")) //结束日期，这里我理解为
	minSizeStr := c.Query("min_size")
	maxSizeStr := c.Query("max_size")
	order := strings.TrimSpace(c.Query("order")) //排序参数

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))       //获取页码参数
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10")) // 获取每页的大小参数

	// 默认的初始化设置
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	var minSize, maxSize int64 // 类型转换
	if minSizeStr != "" {
		if v, err := strconv.ParseInt(minSizeStr, 10, 64); err == nil && v >= 0 {
			minSize = v
		}
	}
	if maxSizeStr != "" {
		if v, err := strconv.ParseInt(maxSizeStr, 10, 64); err == nil && v >= 0 {
			maxSize = v
		}
	}

	db := global.DB.Model(&models.Files{}).Where("user_id = ?", userID) //查询对应的用户id

	if q != "" { //查询文件名
		like := "%" + q + "%"
		db = db.Where("filename LIKE ?", like) //  MySQL 用 LIKE
	}
	if ext != "" { //查询后缀名
		// 允许传 ".pdf" 或 "pdf"
		if !strings.HasPrefix(ext, ".") { //如果没有.后缀名我们就加入并用后缀名查询
			ext = "." + ext
		}
		db = db.Where("LOWER(filename) LIKE ?", "%"+ext)
	}
	//文本内容-文本类型搜索
	if ctPrefix != "" {
		db = db.Where("file_type LIKE ?", ctPrefix+"%") //通配文本前缀
	}
	// 按照日期查询-这里是文件的上传时间
	// 日期：对 CreatedAt 过滤
	if dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			db = db.Where("created_at >= ?", t)
		}
	}
	if dateTo != "" {
		// 包含当天，+1 天再 <
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			db = db.Where("created_at < ?", t.Add(24*time.Hour))
		}
	}
	if minSize > 0 { //按照最小来查询
		db = db.Where("file_size >= ?", minSize)
	}
	if maxSize > 0 { //按照最大查询
		db = db.Where("file_size <= ?", maxSize)
	}

	// 依据前端传来的数据
	switch order {
	case "created_asc":
		db = db.Order("created_at ASC") //上传日期
	case "size_desc":
		db = db.Order("file_size DESC") //文件大小
	case "size_asc":
		db = db.Order("file_size ASC")
	default:
		db = db.Order("created_at DESC")
	}

	var totalsize int64
	if err := db.Count(&totalsize).Error; err != nil { //最终的查询结果
		c.JSON(http.StatusInternalServerError, gin.H{"error": "This file's count failed"})
		return
	}

	var rows []models.Files
	//分页功能
	if err := db.Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil { //计算分页偏移量，实现分页功能
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	items := make([]FileItem, 0, len(rows)) //构建切片，实际上这里的大小为size
	for _, r := range rows {                //每个元素
		items = append(items, FileItem{
			ID:          r.ID,
			Filename:    r.Filename,
			ContentType: r.FileType,
			SizeBytes:   r.FileSize,
			Hash:        r.Hash,
			Path:        r.FilePath, // 相对 key，前端需要拼下载接口或你提供的 URL
			CreatedAt:   r.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, ListFilesResponse{
		Total:    totalsize,
		Page:     page,
		PageSize: size,  //每页的大小
		Items:    items, //数据
	})
}
