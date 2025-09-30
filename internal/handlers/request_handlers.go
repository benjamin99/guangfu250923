package handlers

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"guangfu250923/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type createRequestInput struct {
	Code         string      `json:"code"`
	Name         string      `json:"name" binding:"required"`
	Address      string      `json:"address"`
	Phone        string      `json:"phone"`
	Contact      string      `json:"contact"`
	Status       string      `json:"status"`
	NeededPeople int         `json:"needed_people"`
	Notes        string      `json:"notes"`
	Lng          *float64    `json:"lng"`
	Lat          *float64    `json:"lat"`
	MapLink      string      `json:"map_link"`
	SuppliesAny  interface{} `json:"supplies"`
}

func (h *Handler) CreateRequest(c *gin.Context) {
	var in createRequestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	supplies, err := models.ParseSupplyFlexible(in.SuppliesAny)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.Status == "" {
		in.Status = "pending"
	}
	if in.Contact == "" && in.Phone != "" {
		in.Contact = in.Phone
	}

	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tx begin"})
		return
	}
	defer tx.Rollback(ctx)

	var reqID string
	err = tx.QueryRow(ctx, `insert into requests(code,name,address,phone,contact,status,needed_people,notes,lng,lat,map_link) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) returning id`,
		in.Code, in.Name, in.Address, in.Phone, in.Contact, in.Status, in.NeededPeople, in.Notes, in.Lng, in.Lat, in.MapLink,
	).Scan(&reqID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for i := range supplies {
		s := &supplies[i]
		if s.TotalCount == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "total_count required >0"})
			return
		}
		if s.Unit == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unit required"})
			return
		}
		err = tx.QueryRow(ctx, `insert into supply_items(request_id,tag,name,total_count,received_count,unit) values($1,$2,$3,$4,$5,$6) returning id`,
			reqID, s.Tag, s.Name, s.TotalCount, s.ReceivedCount, s.Unit).Scan(&s.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		s.RequestID = reqID
	}
	if err = tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	createdAt := time.Now().Unix()
	out := models.Request{
		ID:           reqID,
		Code:         in.Code,
		Name:         in.Name,
		Address:      in.Address,
		Phone:        in.Phone,
		Contact:      in.Contact,
		Status:       in.Status,
		NeededPeople: in.NeededPeople,
		Notes:        in.Notes,
		Lng:          in.Lng,
		Lat:          in.Lat,
		MapLink:      in.MapLink,
		CreatedAt:    createdAt,
		Time:         createdAt,
		Supplies:     supplies,
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListRequests(c *gin.Context) {
	status := c.Query("status")
	limit := parsePositiveInt(c.Query("limit"), 20, 1, 200)
	offset := parsePositiveInt(c.Query("offset"), 0, 0, 1000000)
	ctx := context.Background()

	// total count for pagination (optional performance cost)
	var total int
	if status != "" {
		h.pool.QueryRow(ctx, `select count(*) from requests where status=$1`, status).Scan(&total)
	} else {
		h.pool.QueryRow(ctx, `select count(*) from requests`).Scan(&total)
	}

	baseSelect := `select id,code,name,address,phone,contact,status,needed_people,notes,lng,lat,map_link,extract(epoch from created_at)::bigint from requests`
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = h.pool.Query(ctx, baseSelect+` where status=$1 order by created_at desc limit $2 offset $3`, status, limit, offset)
	} else {
		rows, err = h.pool.Query(ctx, baseSelect+` order by created_at desc limit $1 offset $2`, limit, offset)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	reqs := []models.Request{}
	for rows.Next() {
		var r models.Request
		if err = rows.Scan(&r.ID, &r.Code, &r.Name, &r.Address, &r.Phone, &r.Contact, &r.Status, &r.NeededPeople, &r.Notes, &r.Lng, &r.Lat, &r.MapLink, &r.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		reqs = append(reqs, r)
	}

	// Eager load supplies for current page
	if len(reqs) > 0 {
		idSet := make(map[string]*models.Request, len(reqs))
		ids := []any{}
		for i := range reqs {
			idSet[reqs[i].ID] = &reqs[i]
			ids = append(ids, reqs[i].ID)
		}
		placeholders := ""
		for i := range ids {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "$" + strconv.Itoa(i+1)
		}
		supplyRows, err := h.pool.Query(ctx, `select id,request_id,tag,name,total_count,received_count,unit from supply_items where request_id in (`+placeholders+`) order by created_at`, ids...)
		if err == nil {
			for supplyRows.Next() {
				var s models.SupplyItem
				if err = supplyRows.Scan(&s.ID, &s.RequestID, &s.Tag, &s.Name, &s.TotalCount, &s.ReceivedCount, &s.Unit); err != nil {
					break
				}
				if r := idSet[s.RequestID]; r != nil {
					r.Supplies = append(r.Supplies, s)
				}
			}
			supplyRows.Close()
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// JSON-LD style collection wrapper
	lastPage := int(math.Ceil(float64(total)/float64(limit))) - 1
	if lastPage < 0 {
		lastPage = 0
	}
	baseURL := c.Request.URL.Path
	q := c.Request.URL.Query()
	buildLink := func(off int) string {
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(off))
		return baseURL + "?" + q.Encode()
	}
	var nextLink *string
	if offset+limit < total {
		s := buildLink(offset + limit)
		nextLink = &s
	}
	var prevLink *string
	if offset-limit >= 0 {
		s := buildLink(offset - limit)
		prevLink = &s
	}

	c.JSON(http.StatusOK, gin.H{
		"@context":   "https://www.w3.org/ns/hydra/context.jsonld",
		"@type":      "Collection",
		"totalItems": total,
		"member":     reqs,
		"limit":      limit,
		"offset":     offset,
		"next":       nextLink,
		"previous":   prevLink,
		"lastOffset": lastPage * limit,
	})
}

func parsePositiveInt(raw string, def, min, max int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ListSupplies provides a flat list across all supplies with pagination & optional request_id filter.
func (h *Handler) ListSupplies(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 50, 1, 500)
	offset := parsePositiveInt(c.Query("offset"), 0, 0, 1000000)
	requestID := c.Query("request_id")
	ctx := context.Background()
	var total int
	if requestID != "" {
		h.pool.QueryRow(ctx, `select count(*) from supply_items where request_id=$1`, requestID).Scan(&total)
	} else {
		h.pool.QueryRow(ctx, `select count(*) from supply_items`).Scan(&total)
	}
	var rows pgx.Rows
	var err error
	if requestID != "" {
		rows, err = h.pool.Query(ctx, `select id,request_id,tag,name,total_count,received_count,unit from supply_items where request_id=$1 order by created_at desc limit $2 offset $3`, requestID, limit, offset)
	} else {
		rows, err = h.pool.Query(ctx, `select id,request_id,tag,name,total_count,received_count,unit from supply_items order by created_at desc limit $1 offset $2`, limit, offset)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []models.SupplyItem{}
	for rows.Next() {
		var s models.SupplyItem
		if err = rows.Scan(&s.ID, &s.RequestID, &s.Tag, &s.Name, &s.TotalCount, &s.ReceivedCount, &s.Unit); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, s)
	}
	baseURL := c.Request.URL.Path
	q := c.Request.URL.Query()
	build := func(off int) string {
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(off))
		return baseURL + "?" + q.Encode()
	}
	var nextLink *string
	if offset+limit < total {
		s := build(offset + limit)
		nextLink = &s
	}
	var prevLink *string
	if offset-limit >= 0 {
		s := build(offset - limit)
		prevLink = &s
	}
	c.JSON(http.StatusOK, gin.H{
		"@context":   "https://www.w3.org/ns/hydra/context.jsonld",
		"@type":      "Collection",
		"totalItems": total,
		"member":     list,
		"limit":      limit,
		"offset":     offset,
		"next":       nextLink,
		"previous":   prevLink,
	})
}
