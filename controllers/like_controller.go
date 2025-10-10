package controllers

import (
	"net/http"
	"project/global"

	"github.com/gin-gonic/gin"
    "github.com/go-redis/redis"
)

//这里的articleID就是文章的ID
func LikeArticle(c *gin.Context){
    articleID := c.Param("id")
    likeKey := "article:" + articleID +":likes"
    if err:=global.RedisDB.Incr(likeKey).Err();err!=nil{
        c.JSON(http.StatusInternalServerError,gin.H{"error":err.Error()})
        return
    }
    c.JSON(200,gin.H{"message":"Successful liked the article!"})
}

func GetArticleLikes(c *gin.Context){
     articleID := c.Param("id")
     likeKey := "article:" + articleID +":likes"
     likes,err:=global.RedisDB.Get(likeKey).Result()
     if err== redis.Nil{
        likes ="0"
     }else if err!=nil{
        c.JSON(http.StatusInternalServerError,gin.H{"error":err.Error()})
        return
     }
     c.JSON(200,gin.H{"likes":likes})

}