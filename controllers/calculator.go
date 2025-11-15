package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 双栈法计算表达式-中缀表达式转后缀表达式-计算后缀表达式

// CalculatorRequest 计算器请求结构
type CalculatorRequest struct {
	Expression string `json:"expression" binding:"required"` // 计算表达式
}

// CalculatorResponse 计算器响应结构
type CalculatorResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Calculate 处理计算器计算请求
// @Summary 计算器计算
// @Description 支持基本的四则运算，包括小数点计算
// @Security Bearer
// @Tags Calculator
// @Accept json
// @Produce json
// @Param expression body CalculatorRequest true "计算表达式"
// @Success 200 {object} CalculatorResponse
// @Failure 400 {object} map[string]string
// @Router /calculator/calculate [post]
func Calculate(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录,无权限"})
		return
	}
	var req CalculatorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 清理表达式（去除空格）
	expression := strings.ReplaceAll(req.Expression, " ", "")

	// 计算结果
	result, err := evaluateExpression(expression)
	if err != nil {
		c.JSON(http.StatusOK, CalculatorResponse{
			Result: "0",
			Error:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CalculatorResponse{
		Result: result,
	})
}

// evaluateExpression 计算表达式的值
func evaluateExpression(expr string) (string, error) {
	if expr == "" {
		return "0", nil
	}

	// 使用栈来计算表达式
	numStack := []float64{}
	opStack := []rune{}

	i := 0
	for i < len(expr) {
		ch := rune(expr[i])

		if ch >= '0' && ch <= '9' || ch == '.' {
			// 读取完整的数字（包括小数点）
			j := i
			hasDot := false
			for j < len(expr) && (expr[j] >= '0' && expr[j] <= '9' || expr[j] == '.') {
				if expr[j] == '.' {
					if hasDot {
						return "", fmt.Errorf("数字格式错误")
					}
					hasDot = true
				}
				j++
			}

			numStr := expr[i:j]
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return "", fmt.Errorf("无效的数字: %s", numStr)
			}
			numStack = append(numStack, num)
			i = j
		} else if ch == '+' || ch == '-' || ch == '*' || ch == '/' {
			// 处理运算符优先级
			for len(opStack) > 0 && precedence(opStack[len(opStack)-1]) >= precedence(ch) {
				if err := applyOp(&numStack, &opStack); err != nil {
					return "", err
				}
			}
			opStack = append(opStack, ch)
			i++
		} else if ch == '(' {
			opStack = append(opStack, ch)
			i++
		} else if ch == ')' {
			// 处理括号内的运算
			for len(opStack) > 0 && opStack[len(opStack)-1] != '(' {
				if err := applyOp(&numStack, &opStack); err != nil {
					return "", err
				}
			}
			if len(opStack) == 0 {
				return "", fmt.Errorf("括号不匹配")
			}
			opStack = opStack[:len(opStack)-1] // 弹出 '('
			i++
		} else {
			return "", fmt.Errorf("无效的字符: %c", ch)
		}
	}

	// 处理剩余的运算符
	for len(opStack) > 0 {
		if opStack[len(opStack)-1] == '(' || opStack[len(opStack)-1] == ')' {
			return "", fmt.Errorf("括号不匹配")
		}
		if err := applyOp(&numStack, &opStack); err != nil {
			return "", err
		}
	}

	if len(numStack) != 1 {
		return "", fmt.Errorf("表达式格式错误")
	}

	result := numStack[0]

	// 格式化输出（去除不必要的小数位）
	resultStr := strconv.FormatFloat(result, 'f', -1, 64)

	return resultStr, nil
}

// precedence 返回运算符的优先级
func precedence(op rune) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	}
	return 0
}

// applyOp 应用运算符
func applyOp(numStack *[]float64, opStack *[]rune) error {
	if len(*numStack) < 2 {
		return fmt.Errorf("运算数不足")
	}
	if len(*opStack) < 1 {
		return fmt.Errorf("运算符不足")
	}

	b := (*numStack)[len(*numStack)-1]
	*numStack = (*numStack)[:len(*numStack)-1]

	a := (*numStack)[len(*numStack)-1]
	*numStack = (*numStack)[:len(*numStack)-1]

	op := (*opStack)[len(*opStack)-1]
	*opStack = (*opStack)[:len(*opStack)-1]

	var result float64
	switch op {
	case '+':
		result = a + b
	case '-':
		result = a - b
	case '*':
		result = a * b
	case '/':
		if b == 0 {
			return fmt.Errorf("除数不能为零,无效运算")
		}
		result = a / b
	default:
		return fmt.Errorf("无效的运算符: %c", op)
	}

	*numStack = append(*numStack, result)
	return nil
}
