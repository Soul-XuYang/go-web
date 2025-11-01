package utils

import (
	"fmt"
)

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Get_size(data int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	//float无法直接转换为string
	switch {
	case data < KB:
		return fmt.Sprintf("%.2fB ", float64(data)) //保证统一的格式
	case data < MB:
		return fmt.Sprintf("%.2fKB", float64(data))
	case data < GB:
		return fmt.Sprintf("%.2fMB", float64(data))
	case data < TB:
		return fmt.Sprintf("%.2fGB", float64(data))
	default:
		return fmt.Sprintf("%.2fTB", float64(data))
	}
}
