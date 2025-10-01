package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"guangfu250923/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type reportCreateInput struct {
	Name         string  `json:"name" binding:"required"`
	LocationType string  `json:"location_type" binding:"required"`
	Reason       string  `json:"reason" binding:"required"`
	Notes        *string `json:"notes"`
	Status       string  `json:"status" binding:"required"`
	LocationID   string  `json:"location_id" binding:"required"`
}

type reportPatchInput struct {
	Name         *string `json:"name"`
	LocationType *string `json:"location_type"`
	Reason       *string `json:"reason"`
	Notes        *string `json:"notes"`
	Status       *string `json:"status"`
	LocationID   *string `json:"location_id"`
}

func (h *Handler) CreateReport(c *gin.Context) {
	var in reportCreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Basic trim validation
	for field, val := range map[string]string{"name": in.Name, "location_type": in.LocationType, "reason": in.Reason, "status": in.Status, "location_id": in.LocationID} {
		if strings.TrimSpace(val) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": field + " is required"})
			return
		}
	}
	newUUID, err := uuid.NewV7()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate id"})
		return
	}
	id := "incident-" + newUUID.String()
	row := h.pool.QueryRow(context.Background(), `insert into reports(id,name,location_type,reason,notes,status,location_id) values($1,$2,$3,$4,$5,$6,$7) returning id,name,location_type,reason,notes,status,location_id,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint`, id, in.Name, in.LocationType, in.Reason, in.Notes, in.Status, in.LocationID)
	var r models.Report
	var notes *string
	if err := row.Scan(&r.ID, &r.Name, &r.LocationType, &r.Reason, &notes, &r.Status, &r.LocationID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	r.Notes = notes
	c.JSON(http.StatusCreated, r)
}

func (h *Handler) ListReports(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 50, 1, 500)
	offset := parsePositiveInt(c.Query("offset"), 0, 0, 1000000)
	status := strings.TrimSpace(c.Query("status"))
	ctx := context.Background()
	var total int
	countSQL := `select count(*) from reports`
	listSQL := `select id,name,location_type,reason,notes,status,location_id,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from reports`
	args := []interface{}{}
	if status != "" {
		countSQL += " where status=$1"
		listSQL += " where status=$1"
		args = append(args, status)
	}
	listSQL += " order by updated_at desc limit $" + strconv.Itoa(len(args)+1) + " offset $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)
	if err := h.pool.QueryRow(ctx, countSQL, args[:len(args)-2]...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.pool.Query(ctx, listSQL, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []models.Report{}
	for rows.Next() {
		var r models.Report
		var notes *string
		if err := rows.Scan(&r.ID, &r.Name, &r.LocationType, &r.Reason, &notes, &r.Status, &r.LocationID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		r.Notes = notes
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

func (h *Handler) GetReport(c *gin.Context) {
	id := c.Param("id")
	row := h.pool.QueryRow(context.Background(), `select id,name,location_type,reason,notes,status,location_id,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from reports where id=$1`, id)
	var r models.Report
	var notes *string
	if err := row.Scan(&r.ID, &r.Name, &r.LocationType, &r.Reason, &notes, &r.Status, &r.LocationID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	r.Notes = notes
	c.JSON(http.StatusOK, r)
}

func (h *Handler) PatchReport(c *gin.Context) {
	id := c.Param("id")
	var in reportPatchInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	set := []string{}
	args := []interface{}{}
	idx := 1
	add := func(col string, v interface{}) {
		set = append(set, col+"$"+strconv.Itoa(idx))
		args = append(args, v)
		idx++
	}
	if in.Name != nil {
		add("name=", *in.Name)
	}
	if in.LocationType != nil {
		add("location_type=", *in.LocationType)
	}
	if in.Reason != nil {
		add("reason=", *in.Reason)
	}
	if in.Notes != nil {
		add("notes=", *in.Notes)
	}
	if in.Status != nil {
		add("status=", *in.Status)
	}
	if in.LocationID != nil {
		add("location_id=", *in.LocationID)
	}
	if len(set) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields"})
		return
	}
	set = append(set, "updated_at=now()")
	query := "update reports set " + strings.Join(set, ",") + " where id=$" + strconv.Itoa(idx) + " returning id,name,location_type,reason,notes,status,location_id,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint"
	args = append(args, id)
	row := h.pool.QueryRow(context.Background(), query, args...)
	var r models.Report
	var notes *string
	if err := row.Scan(&r.ID, &r.Name, &r.LocationType, &r.Reason, &notes, &r.Status, &r.LocationID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	r.Notes = notes
	c.JSON(http.StatusOK, r)
}

// Utility (reuse from other handlers)
// parsePositiveInt provided by other handler files; keep placeholder reference if needed.
