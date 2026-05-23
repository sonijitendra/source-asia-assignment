package main

import (
	"log"

	"source-asia-assignment/internal/api"
	"source-asia-assignment/internal/catalog"
	"source-asia-assignment/internal/ratelimit"

	"github.com/gin-gonic/gin"
)

func main() {
	limiter := ratelimit.New()
	store := catalog.NewStore()

	reqHandler := api.NewRequestHandler(limiter)
	prodHandler := api.NewProductHandler(store)

	r := gin.Default()

	// Part 1 — rate-limited request endpoint
	r.POST("/request", reqHandler.HandleRequest)
	r.GET("/stats", reqHandler.HandleStats)

	// Part 2 — product catalog
	r.POST("/products", prodHandler.Create)
	r.GET("/products", prodHandler.List)
	r.GET("/products/:id", prodHandler.GetByID)
	r.POST("/products/:id/media", prodHandler.AddMedia)

	log.Println("server listening on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("failed to start server: ", err)
	}
}
