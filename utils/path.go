package utils
import(
    "path/filepath"
    "fmt"
    "strings"
    "os"
)
// 双重检验保证key路径是正常的
func SafeJoinRel(baseRel, key string) (string, error) {
	// 系统转换
	baseRel = filepath.Clean(filepath.FromSlash(baseRel)) 
	key = filepath.Clean(filepath.FromSlash(key))

	if filepath.IsAbs(baseRel) || filepath.IsAbs(key) {  //检查是否为绝对路径
		return "", fmt.Errorf("absolute path not allowed in relative mode")
	}
	if key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator)) { //不能存在..
		return "", fmt.Errorf("path traversal detected")
	}
	full := filepath.Join(baseRel, key) // 拼接
	relBack, err := filepath.Rel(baseRel, full) //如果 full 在 baseRel 之下，它会返回相对于 baseRel 的路径
	if err != nil {
		return "", err
	}
	if relBack == ".." || strings.HasPrefix(relBack, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base")
	}
	return full, nil
}

//获得绝对地址
func GetandBuild_AbsPath(storagePath string) (string, error) {
	abs, err := filepath.Abs(storagePath) //将这个相对路径转换为绝对路径
	if err != nil {
		return "",err
	}
	// 创建路径
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "",err
	}
	return abs,nil
}