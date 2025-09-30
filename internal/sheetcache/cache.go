package sheetcache

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Cache holds data loaded from a Google Sheet tab in memory.
// Data structure: map[rowIndex]map[columnHeader]cellValue
type Cache struct {
	mu      sync.RWMutex
	data    map[string]map[string]string
	headers []string
	updated time.Time
	url     string
	tab     string
	client  *http.Client
}

type Snapshot struct {
	Updated time.Time                    `json:"updated"`
	Headers []string                     `json:"headers"`
	Rows    map[string]map[string]string `json:"rows"`
}

// New creates a cache with given Sheet ID + tab name.
// Public sheet assumed (CSV export). If SHEET_API_KEY env is set and the sheet is private, user must implement API call manually later.
func New(sheetID, tab string) *Cache {
	if sheetID == "" || tab == "" {
		return &Cache{data: map[string]map[string]string{}}
	}
	// CSV export URL pattern (public share: anyone with link)
	url := "https://docs.google.com/spreadsheets/d/" + sheetID + "/gviz/tq?tqx=out:csv&sheet=" + tab
	return &Cache{data: map[string]map[string]string{}, url: url, tab: tab, client: &http.Client{Timeout: 20 * time.Second}}
}

// StartPolling launches background poller (non-blocking). Cancel via context.
func (c *Cache) StartPolling(ctx context.Context, interval time.Duration) {
	if c == nil || c.url == "" || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		c.refreshOnce(context.Background())
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.refreshOnce(context.Background())
			}
		}
	}()
}

func (c *Cache) refreshOnce(ctx context.Context) {
	if c.url == "" {
		return
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		slog.Warn("sheet fetch failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		slog.Warn("sheet non-200", "status", resp.StatusCode)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("sheet read err", "error", err)
		return
	}
	rdr := csv.NewReader(strings.NewReader(string(body)))
	records, err := rdr.ReadAll()
	if err != nil {
		slog.Warn("csv parse err", "error", err)
		return
	}
	if len(records) == 0 {
		return
	}
	headers := records[0]
	data := make(map[string]map[string]string, len(records)-1)
	for i, row := range records[1:] {
		rowMap := map[string]string{}
		for idx, h := range headers {
			if idx < len(row) {
				rowMap[h] = row[idx]
			} else {
				rowMap[h] = ""
			}
		}
		data[strconv.Itoa(i+1)] = rowMap
	}
	c.mu.Lock()
	c.data = data
	c.headers = headers
	c.updated = time.Now()
	c.mu.Unlock()
	slog.Info("sheet cache refreshed", "rows", len(data), "tab", c.tab)
}

// Snapshot returns a copy of current data.
func (c *Cache) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	clone := make(map[string]map[string]string, len(c.data))
	for k, v := range c.data {
		inner := make(map[string]string, len(v))
		for ck, cv := range v {
			inner[ck] = cv
		}
		clone[k] = inner
	}
	headersCopy := append([]string{}, c.headers...)
	return Snapshot{Updated: c.updated, Headers: headersCopy, Rows: clone}
}

// LoadFromFile allows seeding from a local CSV (for testing)
func (c *Cache) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		return errors.New("empty csv")
	}
	headers := recs[0]
	data := map[string]map[string]string{}
	for i, row := range recs[1:] {
		m := map[string]string{}
		for j, h := range headers {
			if j < len(row) {
				m[h] = row[j]
			}
		}
		data[strconv.Itoa(i+1)] = m
	}
	c.mu.Lock()
	c.data = data
	c.headers = headers
	c.updated = time.Now()
	c.mu.Unlock()
	return nil
}
