package models

import (
	"encoding/json"
	"errors"
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
