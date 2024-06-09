package handler

import (
	"context"
	"net/http"
	"ppr-service/models"
	ps "ppr-service/service"

	"github.com/gin-gonic/gin"
)

type PprHandlerImpl interface {
	Login(c *gin.Context)
	SendMessage(c *gin.Context)
}

type PprHandler struct {
	PprService ps.PprService
}

func NewPprHandler(ps ps.PprService) PprHandlerImpl {
	return PprHandler{
		PprService: ps,
	}
}  

// Login
func (h PprHandler) Login(c *gin.Context) {
	ctx := context.Background()
	res, err := h.PprService.Login(ctx)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "image/png", res)
}

// sendMessage
func (h PprHandler) SendMessage(c *gin.Context) {
	req := models.Message{}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := models.ValidateRequest(req.PhoneNumber, req.Message)
	if  err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}

	res, err := h.PprService.SendMessage(req)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "success send message",
		"data" : res,
	})
}