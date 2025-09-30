package models

import (
	"encoding/json"
	"errors"
	"time"
)

type Request struct {
	ID           string       `json:"id"`
	Code         string       `json:"code"`
	Name         string       `json:"name"`
	Address      string       `json:"address"`
	Phone        string       `json:"phone"`
	Contact      string       `json:"contact"`
	Status       string       `json:"status"`
	NeededPeople int          `json:"needed_people"`
	Notes        string       `json:"notes"`
	Lng          *float64     `json:"lng"`
	Lat          *float64     `json:"lat"`
	MapLink      string       `json:"map_link"`
	CreatedAt    int64        `json:"created_at"`
	Time         int64        `json:"time"` // alias of created_at for simplified external spec
	Supplies     []SupplyItem `json:"supplies"`
}

type SupplyItem struct {
	ID            string `json:"id"`
	RequestID     string `json:"request_id"`
	Tag           string `json:"tag"`
	Name          string `json:"name"`
	TotalCount    int    `json:"total_count"`
	ReceivedCount int    `json:"received_count"`
	Unit          string `json:"unit"`
}

// UnmarshalJSON allows both received_count (preferred) & legacy typo recieved_count.
func (s *SupplyItem) UnmarshalJSON(b []byte) error {
	type Alias SupplyItem
	var a Alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err == nil {
		if v, ok := raw["received_count"]; ok {
			_ = json.Unmarshal(v, &a.ReceivedCount)
		}
	}
	*s = SupplyItem(a)
	return nil
}

// Parse flexible supplies input: can be object or array.
func ParseSupplyFlexible(raw interface{}) ([]SupplyItem, error) {
	if raw == nil {
		return []SupplyItem{}, nil
	}
	// Try array first
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var arr []SupplyItem
	if err = json.Unmarshal(b, &arr); err == nil {
		return arr, nil
	}
	// Try single object
	var one SupplyItem
	if err2 := json.Unmarshal(b, &one); err2 == nil {
		return []SupplyItem{one}, nil
	}
	return nil, errors.New("invalid supplies format, need object or array")
}

// VolunteerOrganization represents volunteer_organizations table.
type VolunteerOrganization struct {
	ID                 string     `json:"id"`
	LastUpdated        *time.Time `json:"last_updated"`
	RegistrationStatus string     `json:"registration_status"`
	OrganizationNature string     `json:"organization_nature"`
	OrganizationName   string     `json:"organization_name"`
	Coordinator        string     `json:"coordinator"`
	ContactInfo        string     `json:"contact_info"`
	RegistrationMethod string     `json:"registration_method"`
	ServiceContent     string     `json:"service_content"`
	MeetingInfo        string     `json:"meeting_info"`
	Notes              string     `json:"notes"`
	ImageURL           *string    `json:"image_url"`
}

// SuppliesOverview row (from view supplies_overview)
type SuppliesOverview struct {
	ItemID                  string  `json:"item_id"`
	RequestID               string  `json:"request_id"`
	Org                     string  `json:"org"`
	Address                 string  `json:"address"`
	Phone                   string  `json:"phone"`
	Status                  string  `json:"status"`
	IsCompleted             bool    `json:"is_completed"`
	HasMedical              bool    `json:"has_medical"`
	CreatedAt               int64   `json:"created_at"`
	UpdatedAt               int64   `json:"updated_at"`
	ItemName                string  `json:"item_name"`
	ItemType                string  `json:"item_type"`
	ItemNeed                int     `json:"item_need"`
	ItemGot                 int     `json:"item_got"`
	ItemUnit                string  `json:"item_unit"`
	ItemStatus              string  `json:"item_status"`
	DeliveryID              *string `json:"delivery_id"`
	DeliveryTimestamp       *int64  `json:"delivery_timestamp"`
	DeliveryQuantity        *int    `json:"delivery_quantity"`
	DeliveryNotes           *string `json:"delivery_notes"`
	TotalItemsInRequest     int     `json:"total_items_in_request"`
	CompletedItemsInRequest int     `json:"completed_items_in_request"`
	PendingItemsInRequest   int     `json:"pending_items_in_request"`
	TotalRequests           int     `json:"total_requests"`
	ActiveRequests          int     `json:"active_requests"`
	CompletedRequests       int     `json:"completed_requests"`
	CancelledRequests       int     `json:"cancelled_requests"`
	TotalItems              int     `json:"total_items"`
	CompletedItems          int     `json:"completed_items"`
	PendingItems            int     `json:"pending_items"`
	UrgentRequests          int     `json:"urgent_requests"`
	MedicalRequests         int     `json:"medical_requests"`
}

// Shelter represents shelters table row
type Shelter struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Location         string   `json:"location"`
	Phone            string   `json:"phone"`
	Link             *string  `json:"link"`
	Status           string   `json:"status"`
	Capacity         *int     `json:"capacity"`
	CurrentOccupancy *int     `json:"current_occupancy"`
	AvailableSpaces  *int     `json:"available_spaces"`
	Facilities       []string `json:"facilities"`
	ContactPerson    *string  `json:"contact_person"`
	Notes            *string  `json:"notes"`
	Coordinates      *struct {
		Lat *float64 `json:"lat"`
		Lng *float64 `json:"lng"`
	} `json:"coordinates"`
	OpeningHours *string `json:"opening_hours"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
}
