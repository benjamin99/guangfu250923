package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"time"

	"guangfu250923/internal/config"
	"guangfu250923/internal/db"
	"guangfu250923/internal/handlers"
	"guangfu250923/internal/sheetcache"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	pool, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer pool.Close()

	slog.Info("database connected", "cfg", cfg.DBHost+":"+cfg.DBPort+"/"+cfg.DBName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// Sheet cache
	sheetCache := sheetcache.New(cfg.SheetID, cfg.SheetTab)
	pollCtx, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()
	sheetCache.StartPolling(pollCtx, cfg.SheetInterval)
	r.GET("/sheet/snapshot", func(c *gin.Context) { c.JSON(http.StatusOK, sheetCache.Snapshot()) })

	h := handlers.New(pool)
	r.POST("/requests", h.CreateRequest)
	r.GET("/requests", h.ListRequests)
	r.POST("/supplies/distribute", h.DistributeSupplies)
	r.GET("/supplies", h.ListSupplies)
	r.GET("/supplies_overview", h.ListSuppliesOverview)
	r.POST("/shelters", h.CreateShelter)
	r.GET("/shelters", h.ListShelters)
	r.GET("/shelters/:id", h.GetShelter)
	r.PATCH("/shelters/:id", h.PatchShelter)
	r.POST("/volunteer_organizations", h.CreateVolunteerOrg)
	r.GET("/volunteer_organizations", h.ListVolunteerOrgs)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	log.Printf("server listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
