package gee

import (
	"fmt"
	"net/http"
	"strings"
)

type router struct { // 路由表 包含一个
	roots    map[string]*node       // 每个 HTTP 方法一棵 Trie 根节点，如 roots["GET"]-注意存的是节点地址
	handlers map[string]HandlerFunc //  每个 HTTP 方法一棵 Trie 根节点，如 roots["GET"]-注意存的是节点地址
}

func newRouter() *router {
	return &router{
		roots:    make(map[string]*node),
		handlers: make(map[string]HandlerFunc),
	}
}

// 解析路由-这里只区别*和非*的区别
func parsePattern(pattern string) []string {
	vs := strings.Split(pattern, "/")

	parts := make([]string, 0)
	for _, item := range vs {
		if item != "" {
			parts = append(parts, item)
			if item[0] == '*' {
				break
			}
		}
	}
	return parts
}

// 添加对应的路由映射
// 响应函数是方法和字符串共同响应的，而字符串则是通过解析得到并填入前缀树里的
func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
	parts := parsePattern(pattern)

	key := method + "-" + pattern
	_, ok := r.roots[method]
	if !ok {
		r.roots[method] = &node{} //如果没有这个method，则创建一个节点
	}
	r.roots[method].insert(pattern, parts, 0) //对应的这个方法添加一其字符串
	r.handlers[key] = handler                 //对应的这个方法+字符串添加一函数
}

func (r *router) getRoute(method string, path string) (*node, map[string]string) {  //这里的映射都是来自于路由表
	// searchParts是传入路径的分割字符串数组  
	searchParts := parsePattern(path) // 初始化创建，这里用解析来替代
	params := make(map[string]string) // 创建哈希表对应的的名称是:后的字符
	root, ok := r.roots[method]       //查找前缀表里是否存在这个映射方法
	if !ok {
		return nil, nil
	}
	n := root.search(searchParts, 0) //对应搜索前缀树
	if n != nil {
		fmt.Println("后端的路由表匹配成功!")
		parts := parsePattern(n.pattern) //解析前缀树里对应的字符串
		for index, part := range parts { //遍历解析出来的字符数组
			// 对应:的映射-单端映射
			if part[0] == ':' { // 单端数据
				params[part[1:]] = searchParts[index] //获得对应的索引数据
			}
			// 这里是对*后的字符的全映射
			if part[0] == '*' && len(part) > 1 { // 多段数据
				params[part[1:]] = strings.Join(searchParts[index:], "/") // 这里是把全部的映射填入哈希表里
				break
			}
		}
		//上述的哈希表是在映射框架下获取其值
		return n, params
	}

	return nil, nil
}

func (r *router) getRoutes(method string) []*node {
	root, ok := r.roots[method]
	if !ok {
		fmt.Printf("不存在%s方法!\n", method)
		return nil
	} //先查哈希表里是否存在这个方法
	fmt.Printf("%s下的所有方法:\n", method)
	nodes := make([]*node, 0) //创建一个节点数组
	root.travel(&nodes)       //收集其前缀树下的所有规则-即完结的所有节点
	return nodes
}

// 请求分发口
func (r *router) handle(c *Context) { // 获取上下文信息
	n, params := r.getRoute(c.Method, c.Path) //获取命中的 Trie 终点节点和哈希表
	if n != nil {
		c.Params = params //这里的映射按命中的路由模板（:param / param）从请求路径中解析出的键值
		key := c.Method + "-" + n.pattern
		r.handlers[key](c) // 找到对应的函数并执行
	} else {
		c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
	}
}
