package log

type FileStats struct { //各个文件的状态
	Files int
	Lines int
	Size  int64
}

type CodeCounter struct {
	FileTypes map[string][]string
	IgnoreDirs map[string]bool
	Results   map[string]FileStats
}