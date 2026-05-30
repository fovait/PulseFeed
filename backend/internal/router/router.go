package router

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetRouter() *gin.Engine {
	r := gin.Default()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Printf("SetTrustedProxies failed: %v", err)
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(
			http.StatusOK,
			gin.H{
				"status": "ok",
			},
		)
	})

	r.Static("/static", "./.run/uploads")

	return r
}
