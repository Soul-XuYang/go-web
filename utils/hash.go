package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
    "errors"
    "fmt"
)


var (
	ErrSizeExceeded = errors.New("file size exceeded limit")
	ErrSizeMismatch = errors.New("file size mismatched expected size")
)
func CalculateHash(file io.Reader) (string, error) {  //接收一个 io.Reader 类型的参数，这意味着它可以处理任何实现了 Reader 接口的对象
	h := sha256.New() // 创建一个新的 SHA256 哈希计算器
	_, err := io.Copy(h, file) // 将文件内容复制到哈希计算器中，这一步会自动计算哈希值
	return hex.EncodeToString(h.Sum(nil)), err //将文件内容复制到哈希计算器中，这一步会自动计算哈希值
}



func CopyWithHash(dst io.Writer, src io.Reader, maxSize, expectedSize int64) (string, int64, error) {
	hasher := sha256.New() // 创建一个新的 SHA256 哈希计算器
	w := io.MultiWriter(dst, hasher) // 创建一个多路写入器 一是文件存储二是哈希计算器

	// 读取上限保护
	var r io.Reader = src  // 声明一个Reader类型的变量r，初始值指向原始数据源src,它可以是任何实现了io.Reader接口的对象，比如文件、网络连接等
	if maxSize > 0 {
		r = io.LimitReader(src, maxSize+1) //使得有了限制条件
	}

	written, err := io.Copy(w, r)
	if err != nil {
		return "", written, fmt.Errorf("this file's copy failed: %w", err)
	}

	// 超限判断（用 > maxSize，因为我们读了 maxSize+1 触发检测）
	if maxSize > 0 && written > maxSize {
		return "", written, ErrSizeExceeded
	}
	// 期望大小不匹配
	if expectedSize > 0 && written != expectedSize {
		return "", written, ErrSizeMismatch
	}

	sum := hasher.Sum(nil)
	return hex.EncodeToString(sum), written, nil
}