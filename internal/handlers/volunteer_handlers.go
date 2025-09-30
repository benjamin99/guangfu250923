package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"guangfu250923/internal/models"

	"github.com/gin-gonic/gin"
)

// 對應到 https://github.com/carolchu1208/Hualian-Typhoon-Rescue-Site-Backend-Team/blob/main/spec.md#volunteer_organizations

type createVolunteerOrgInput struct {
	RegistrationStatus string  `json:"registration_status"`
	OrganizationNature string  `json:"organization_nature"`
	OrganizationName   string  `json:"organization_name" binding:"required"`
	Coordinator        string  `json:"coordinator"`
	ContactInfo        string  `json:"contact_info"`
	RegistrationMethod string  `json:"registration_method"`
	ServiceContent     string  `json:"service_content"`
	MeetingInfo        string  `json:"meeting_info"`
	Notes              string  `json:"notes"`
	ImageURL           *string `json:"image_url"`
}

func (h *Handler) CreateVolunteerOrg(c *gin.Context) {
	var in createVolunteerOrgInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()
	var id string
	var lastUpdated time.Time
	err := h.pool.QueryRow(ctx, `insert into volunteer_organizations(last_updated,registration_status,organization_nature,organization_name,coordinator,contact_info,registration_method,service_content,meeting_info,notes,image_url) values(now(),$1,$2,$3,$4,$5,$6,$7,$8,$9,$10) returning id,last_updated`,
		in.RegistrationStatus, in.OrganizationNature, in.OrganizationName, in.Coordinator, in.ContactInfo, in.RegistrationMethod, in.ServiceContent, in.MeetingInfo, in.Notes, in.ImageURL,
	).Scan(&id, &lastUpdated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := models.VolunteerOrganization{ID: id, LastUpdated: &lastUpdated, RegistrationStatus: in.RegistrationStatus, OrganizationNature: in.OrganizationNature, OrganizationName: in.OrganizationName, Coordinator: in.Coordinator, ContactInfo: in.ContactInfo, RegistrationMethod: in.RegistrationMethod, ServiceContent: in.ServiceContent, MeetingInfo: in.MeetingInfo, Notes: in.Notes, ImageURL: in.ImageURL}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListVolunteerOrgs(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 20, 1, 200)
	offset := parsePositiveInt(c.Query("offset"), 0, 0, 1000000)
	ctx := context.Background()
	var total int
	h.pool.QueryRow(ctx, `select count(*) from volunteer_organizations`).Scan(&total)
	rows, err := h.pool.Query(ctx, `select id,last_updated,registration_status,organization_nature,organization_name,coordinator,contact_info,registration_method,service_content,meeting_info,notes,image_url from volunteer_organizations order by coalesce(last_updated, id::text)::text desc limit $1 offset $2`, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []models.VolunteerOrganization{}
	for rows.Next() {
		var vo models.VolunteerOrganization
		if err = rows.Scan(&vo.ID, &vo.LastUpdated, &vo.RegistrationStatus, &vo.OrganizationNature, &vo.OrganizationName, &vo.Coordinator, &vo.ContactInfo, &vo.RegistrationMethod, &vo.ServiceContent, &vo.MeetingInfo, &vo.Notes, &vo.ImageURL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, vo)
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
	c.JSON(http.StatusOK, gin.H{
		"@context":   "https://www.w3.org/ns/hydra/context.jsonld",
		"@type":      "Collection",
		"totalItems": total,
		"member":     list,
		"limit":      limit,
		"offset":     offset,
		"next":       next,
		"previous":   prev,
	})
}
