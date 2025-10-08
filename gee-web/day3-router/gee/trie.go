package gee

import (
	"fmt"
	"strings"
)

type node struct {
	pattern  string  // 只有当一条路由在此节点“完整结束”时才会写入 - 用于判断“这里是不是一个可命中的终点
	part     string  // 当前节点代表的“路径片段”（按 / 切分后的某一段）。
	children []*node // 子节点，存储下一级的节点
	isWild   bool    // 该片段是否为动态匹配（以 : 或 * 开头）
}

func (n *node) String() string {
	return fmt.Sprintf("node{pattern=%s, part=%s, isWild=%t}", n.pattern, n.part, n.isWild)  // 返回字符串
}

func (n *node) insert(pattern string, parts []string, height int) {
	if len(parts) == height { //当走到 height == len(parts) 时，说明整条路由的所有片段都“落位”了；此时把完整路由模板写到 n.pattern，标记“此处为可命中的终点”（以后匹配到这里才算一条有效路由）。
		n.pattern = pattern
		return
	}

	part := parts[height]
	child := n.matchChild(part)
	//取当前层要插入的片段 part := parts[height]，在子节点里找可复用的分支：
	// matchChild(part) 会优先复用同名静态段或已存在的动态段（isWild==true）；
	// 若没有，则新建子节点，并依据 part 首字符设置 isWild：
	// : 或 * → true（动态匹配）、 其他 → false（字面匹配）
	if child == nil {
		child = &node{part: part, isWild: part[0] == ':' || part[0] == '*'} // 在这里，这套路由里动态片段就是用 : 和 * 来表示的前缀的
		n.children = append(n.children, child)
	}
	child.insert(pattern, parts, height+1) //递归，深度+1
}

func (n *node) search(parts []string, height int) *node {
	if len(parts) == height || strings.HasPrefix(n.part, "*") {   // 这里时遇到*则动态停止了 
		if n.pattern == "" { //只有当这里是一个“可终止”的已注册路由时，才把整条路由模板写进来（如 "/hello/:name"）。否则 pattern==""。
			return nil // 终止了，但是原先的pattern没有整个说明没有匹配到，返回nil
		}
		return n
	}

	part := parts[height]
	children := n.matchChildren(part) //当前子节点，如果有相同的前缀，则返回子节点，否则返回nil
	if children == nil {
		return nil
	}
	// 当对一个nil的slice或map进行range操作时
	for _, child := range children { //回溯DFS递归
		result := child.search(parts, height+1)
		if result != nil {
			return result
		}
	}

	return nil
}

func (n *node) travel(list *([]*node)) { //list 用来收集“已注册路由的终止节点”（pattern≠""）
	if n.pattern != "" {
		*list = append(*list, n)
	}
	for _, child := range n.children { // 这个是回溯DFS递归的过程
		child.travel(list)
	}
}

// 查找当前子节点
func (n *node) matchChild(part string) *node { //寻找当前节点的子节点有没有匹配的子节点的数据
	for _, child := range n.children {
		if child.part == part || child.isWild {
			return child
		}
	}
	return nil //没有找到匹配的子节点-返回为nil
}

func (n *node) matchChildren(part string) []*node { // 返回的是一个列表
	nodes := make([]*node, 0)
	for _, child := range n.children { // 子列表一致
		if child.part == part || child.isWild {  //如果子节点是动态匹配的，那么就返回这个子节点
			nodes = append(nodes, child)
		}
	}
	return nodes
}
