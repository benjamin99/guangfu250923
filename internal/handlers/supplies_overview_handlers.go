package handlers

import (
	"context"
	"net/http"
	"strconv"

	"guangfu250923/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// ListSuppliesOverview returns rows from supplies_overview view with pagination and optional status filter.
func (h *Handler) ListSuppliesOverview(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 50, 1, 500)
	offset := parsePositiveInt(c.Query("offset"), 0, 0, 1000000)
	status := c.Query("status")
	ctx := context.Background()
	// Count: approximate using subquery (could be expensive).
	var total int
	if status != "" {
		h.pool.QueryRow(ctx, `select count(*) from supplies_overview where status=$1`, status).Scan(&total)
	} else {
		h.pool.QueryRow(ctx, `select count(*) from supplies_overview`).Scan(&total)
	}
	var rows pgx.Rows
	var err error
	base := `select item_id, request_id, org, address, phone, status, is_completed, has_medical, created_at, updated_at, item_name, item_type, item_need, item_got, item_unit, item_status, delivery_id, delivery_timestamp, delivery_quantity, delivery_notes, total_items_in_request, completed_items_in_request, pending_items_in_request, total_requests, active_requests, completed_requests, cancelled_requests, total_items, completed_items, pending_items, urgent_requests, medical_requests from supplies_overview`
	if status != "" {
		rows, err = h.pool.Query(ctx, base+` where status=$1 order by created_at desc limit $2 offset $3`, status, limit, offset)
	} else {
		rows, err = h.pool.Query(ctx, base+` order by created_at desc limit $1 offset $2`, limit, offset)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []models.SuppliesOverview{}
	for rows.Next() {
		var r models.SuppliesOverview
		var delTs *int64
		if err = rows.Scan(&r.ItemID, &r.RequestID, &r.Org, &r.Address, &r.Phone, &r.Status, &r.IsCompleted, &r.HasMedical, &r.CreatedAt, &r.UpdatedAt, &r.ItemName, &r.ItemType, &r.ItemNeed, &r.ItemGot, &r.ItemUnit, &r.ItemStatus, &r.DeliveryID, &delTs, &r.DeliveryQuantity, &r.DeliveryNotes, &r.TotalItemsInRequest, &r.CompletedItemsInRequest, &r.PendingItemsInRequest, &r.TotalRequests, &r.ActiveRequests, &r.CompletedRequests, &r.CancelledRequests, &r.TotalItems, &r.CompletedItems, &r.PendingItems, &r.UrgentRequests, &r.MedicalRequests); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if delTs != nil {
			r.DeliveryTimestamp = delTs
		}
		list = append(list, r)
	}
	baseURL := c.Request.URL.Path
	q := c.Request.URL.Query()
	build := func(off int) string {
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(off))
		return baseURL + "?" + q.Encode()
	}
	var next *string
	if offset+limit < total {
		s := build(offset + limit)
		next = &s
	}
	var prev *string
	if offset-limit >= 0 {
		s := build(offset - limit)
		prev = &s
	}
	c.JSON(http.StatusOK, gin.H{"@context": "https://www.w3.org/ns/hydra/context.jsonld", "@type": "Collection", "totalItems": total, "member": list, "limit": limit, "offset": offset, "next": next, "previous": prev})
}
