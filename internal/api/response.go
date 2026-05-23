package api

import "github.com/gin-gonic/gin"

// Thin wrappers for a consistent JSON envelope across all endpoints.

func jsonOK(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{"data": data})
}

func jsonError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}
