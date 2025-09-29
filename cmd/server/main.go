package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"guangfu250923/internal/config"
	"guangfu250923/internal/db"
	"guangfu250923/internal/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	pool, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	h := handlers.New(pool)
	r.POST("/requests", h.CreateRequest)
	r.GET("/requests", h.ListRequests)
	r.POST("/supplies/distribute", h.DistributeSupplies)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	log.Printf("server listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
