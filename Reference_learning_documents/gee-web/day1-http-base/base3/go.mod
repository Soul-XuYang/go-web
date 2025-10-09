module example

go 1.13

require gee v0.0.0 // 你可以把这个版本号视为占位符

replace gee => ./gee // 这行会将 'gee' 替换为本地的 ./gee 目录
