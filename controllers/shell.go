package controllers

import "github.com/gin-gonic/gin"

func ShellPage(c *gin.Context){
    c.HTML(200, "shell.html", nil)
}