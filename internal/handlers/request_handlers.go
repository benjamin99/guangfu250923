package handlers

import (
	"context"
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
		Supplies:     supplies,
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListRequests(c *gin.Context) {
	status := c.Query("status")
	ctx := context.Background()
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = h.pool.Query(ctx, `select id,code,name,address,phone,contact,status,needed_people,notes,lng,lat,map_link,extract(epoch from created_at)::bigint from requests where status=$1 order by created_at desc`, status)
	} else {
		rows, err = h.pool.Query(ctx, `select id,code,name,address,phone,contact,status,needed_people,notes,lng,lat,map_link,extract(epoch from created_at)::bigint from requests order by created_at desc`)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	reqs := []models.Request{}
	for rows.Next() {
		var r models.Request
		err = rows.Scan(&r.ID, &r.Code, &r.Name, &r.Address, &r.Phone, &r.Contact, &r.Status, &r.NeededPeople, &r.Notes, &r.Lng, &r.Lat, &r.MapLink, &r.CreatedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		reqs = append(reqs, r)
	}
	// load supplies by request ids
	if len(reqs) > 0 {
		idSet := make(map[string]*models.Request, len(reqs))
		ids := []any{}
		for i := range reqs {
			idSet[reqs[i].ID] = &reqs[i]
			ids = append(ids, reqs[i].ID)
		}
		// Build IN clause dynamically (small scale acceptable)
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
	c.JSON(http.StatusOK, reqs)
}
