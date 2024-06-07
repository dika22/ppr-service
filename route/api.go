package route

import (
	handler "ppr-service/handler"
	"ppr-service/service"

	"github.com/gin-gonic/gin"
)

func RouterGroup(router *gin.Engine, ps service.PprService) {
	h := handler.PprHandler{
		PprService: ps,
	}
	
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.GET("/v1/login", h.Login)
	router.POST("/v1/send-message", h.SendMessage)
}