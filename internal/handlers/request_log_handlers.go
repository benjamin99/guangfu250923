package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type RequestLog struct {
	ID         string            `json:"id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      *string           `json:"query"`
	IP         *string           `json:"ip"`
	Headers    map[string]string `json:"headers"`
	StatusCode *int              `json:"status_code"`
	Error      *string           `json:"error"`
	DurationMS *int              `json:"duration_ms"`
	CreatedAt  int64             `json:"created_at"`
}

func (h *Handler) ListRequestLogs(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 100, 1, 500)
	offset := parsePositiveInt(c.Query("offset"), 0, 0, 1000000)
	ctx := context.Background()
	var total int
	if err := h.pool.QueryRow(ctx, `select count(*) from request_logs`).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.pool.Query(ctx, `select id,method,path,query,ip,headers,status_code,error,duration_ms,extract(epoch from created_at)::bigint from request_logs order by created_at desc limit $1 offset $2`, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []RequestLog{}
	for rows.Next() {
		var rl RequestLog
		var headersJSON map[string]string
		if err := rows.Scan(&rl.ID, &rl.Method, &rl.Path, &rl.Query, &rl.IP, &headersJSON, &rl.StatusCode, &rl.Error, &rl.DurationMS, &rl.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rl.Headers = headersJSON
		list = append(list, rl)
	}
	base := c.Request.URL.Path
	q := c.Request.URL.Query()
	build := func(off int) string {
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(off))
		return base + "?" + q.Encode()
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
