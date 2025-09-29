package handlers

import (
	"context"
	"fmt"
	"net/http"

	"guangfu250923/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type distributionInput struct {
	ID    string `json:"id" binding:"required"`
	Count int    `json:"count" binding:"required"`
}

func (h *Handler) DistributeSupplies(c *gin.Context) {
	var inputs []distributionInput
	if err := c.ShouldBindJSON(&inputs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(inputs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty array"})
		return
	}

	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tx begin"})
		return
	}
	defer tx.Rollback(ctx)
	updated := []models.SupplyItem{}
	for _, in := range inputs {
		var s models.SupplyItem
		// lock row
		err = tx.QueryRow(ctx, `select id,request_id,tag,name,total_count,received_count,unit from supply_items where id=$1 for update`, in.ID).Scan(&s.ID, &s.RequestID, &s.Tag, &s.Name, &s.TotalCount, &s.ReceivedCount, &s.Unit)
		if err != nil {
			if err == pgx.ErrNoRows {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("supply %s not found", in.ID)})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if in.Count <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "count must be >0"})
			return
		}
		if s.ReceivedCount+in.Count > s.TotalCount {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("exceed total for %s", s.ID)})
			return
		}
		s.ReceivedCount += in.Count
		_, err = tx.Exec(ctx, `update supply_items set received_count=$1 where id=$2`, s.ReceivedCount, s.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updated = append(updated, s)
	}
	if err = tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}
