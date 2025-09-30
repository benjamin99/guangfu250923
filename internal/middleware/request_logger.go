package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// responseRecorder captures status and body (if needed truncated) for logging.
type responseRecorder struct {
	gin.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }
func (r *responseRecorder) Write(b []byte) (int, error) {
	// copy to buffer (limit to 256KB)
	if r.buf.Len() < 256*1024 {
		max := 256*1024 - r.buf.Len()
		if len(b) > max {
			r.buf.Write(b[:max])
		} else {
			r.buf.Write(b)
		}
	}
	return r.ResponseWriter.Write(b)
}

// RequestLogger returns a gin middleware that logs request metadata + error info into request_logs table.
// It stores headers (all) as JSON, client IP (as seen by gin), status code, and any error message set in context.
func RequestLogger(pool *pgxpool.Pool, maxHeaderBytes int) gin.HandlerFunc {
	if maxHeaderBytes <= 0 {
		maxHeaderBytes = 16 * 1024
	}
	return func(c *gin.Context) {
		start := time.Now()
		recorder := &responseRecorder{ResponseWriter: c.Writer, status: 200}
		c.Writer = recorder

		var rawBody []byte
		if c.Request.Body != nil && (c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPatch) {
			// read and replace body so handler can still consume
			bodyBytes, _ := io.ReadAll(io.LimitReader(c.Request.Body, 256*1024))
			rawBody = bodyBytes
			c.Request.Body.Close()
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		var originalData json.RawMessage
		var resourceID *string
		// For PATCH fetch original row (best-effort) based on path pattern /:resource/:id
		if c.Request.Method == http.MethodPatch {
			if id := extractIDFromPath(c.FullPath(), c.Request.URL.Path); id != "" {
				resourceID = &id
				if data := fetchOriginal(c, pool, c.FullPath(), id); len(data) > 0 {
					originalData = data
				}
			}
		}

		// Read headers map
		headersMap := make(map[string]string, len(c.Request.Header))
		for k, v := range c.Request.Header {
			if len(v) == 0 {
				continue
			}
			joined := v[0]
			if len(joined) > maxHeaderBytes {
				joined = joined[:maxHeaderBytes]
			}
			headersMap[k] = joined
		}

		// Capture body only if it is small (optional); skipped now to avoid consuming stream.

		c.Next()

		dur := time.Since(start)
		var errMsg string
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}

		// Serialize headers
		headersJSON, _ := jsonMarshal(headersMap)

		// Insert asynchronously (fire and forget)
		go func(method, path, rawQuery, ip string, status int, errText string, headers []byte, took time.Duration, reqBody []byte, orig json.RawMessage, result json.RawMessage, resID *string) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var rid interface{}
			if resID != nil { rid = *resID } else { rid = nil }
			_, _ = pool.Exec(ctx, `insert into request_logs(method,path,query,ip,headers,status_code,error,duration_ms,request_body,original_data,result_data,resource_id) values($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12)`,
				method, path, rawQuery, ip, string(headers), status, nullIfEmpty(errText), int(took.Milliseconds()), bytesOrNull(reqBody), jsonOrNull(orig), jsonOrNull(result), rid)
		}(c.Request.Method, c.FullPath(), c.Request.URL.RawQuery, clientIP(c), recorder.status, errMsg, headersJSON, dur, rawBody, originalData, recorder.buf.Bytes(), resourceID)
	}
}

// Helper functions (minimal to avoid extra deps)

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func bytesOrNull(b []byte) *string {
	if len(b) == 0 { return nil }
	s := string(b)
	return &s
}

func jsonOrNull(b []byte) *string {
	if len(b) == 0 { return nil }
	s := string(b)
	return &s
}

var idPattern = regexp.MustCompile(`(?i)^[0-9a-f-]{16,36}$`)

// extractIDFromPath tries to correlate the parameterized gin route with actual path to capture :id value.
func extractIDFromPath(fullPathPattern, actual string) string {
	// gin's c.FullPath() returns pattern like /shelters/:id
	// actual path is e.g. /shelters/uuid-value
	if !strings.Contains(fullPathPattern, ":id") { return "" }
	partsP := strings.Split(fullPathPattern, "/")
	partsA := strings.Split(actual, "/")
	if len(partsP) != len(partsA) { return "" }
	for i := range partsP {
		if partsP[i] == ":id" {
			cand := partsA[i]
			if idPattern.MatchString(cand) { return cand }
			return ""
		}
	}
	return ""
}

// fetchOriginal best-effort fetch row before PATCH. Only implemented for select known resources.
func fetchOriginal(c *gin.Context, pool *pgxpool.Pool, pattern, id string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	var sql string
	switch pattern {
	case "/shelters/:id":
		sql = "select row_to_json(t) from (select id,name,location,phone,link,status,capacity,current_occupancy,available_spaces,facilities,contact_person,notes,lat,lng,opening_hours,extract(epoch from created_at)::bigint as created_at,extract(epoch from updated_at)::bigint as updated_at from shelters where id=$1) t"
	case "/medical_stations/:id":
		sql = "select row_to_json(t) from (select id,name,station_type,address,phone,status,services,lat,lng,notes,extract(epoch from created_at)::bigint as created_at,extract(epoch from updated_at)::bigint as updated_at from medical_stations where id=$1) t"
	case "/mental_health_resources/:id":
		sql = "select row_to_json(t) from (select id,name,organization,duration_type,service_format,contact,website,status,lat,lng,notes,extract(epoch from created_at)::bigint as created_at,extract(epoch from updated_at)::bigint as updated_at from mental_health_resources where id=$1) t"
	case "/accommodations/:id":
		sql = "select row_to_json(t) from (select id,name,address,phone,township,status,capacity,available,has_vacancy,lat,lng,notes,extract(epoch from created_at)::bigint as created_at,extract(epoch from updated_at)::bigint as updated_at from accommodations where id=$1) t"
	case "/shower_stations/:id":
		sql = "select row_to_json(t) from (select id,name,address,phone,facility_type,is_free,requires_appointment,status,opening_hours,lat,lng,notes,extract(epoch from created_at)::bigint as created_at,extract(epoch from updated_at)::bigint as updated_at from shower_stations where id=$1) t"
	case "/water_refill_stations/:id":
		sql = "select row_to_json(t) from (select id,name,address,phone,water_type,is_free,accessibility,status,opening_hours,lat,lng,notes,extract(epoch from created_at)::bigint as created_at,extract(epoch from updated_at)::bigint as updated_at from water_refill_stations where id=$1) t"
	case "/restrooms/:id":
		sql = "select row_to_json(t) from (select id,name,address,phone,facility_type,opening_hours,is_free,male_units,female_units,unisex_units,accessible_units,has_water,has_lighting,status,cleanliness,last_cleaned,facilities,distance_to_disaster_area,notes,info_source,lat,lng,extract(epoch from created_at)::bigint as created_at,extract(epoch from updated_at)::bigint as updated_at from restrooms where id=$1) t"
	default:
		return nil
	}
	var raw *string
	if err := pool.QueryRow(ctx, sql, id).Scan(&raw); err != nil || raw == nil { return nil }
	return []byte(*raw)
}

func clientIP(c *gin.Context) string {
	// Priority order (Cloudflare aware):
	// 1. CF-Connecting-IP
	// 2. True-Client-IP
	// 3. X-Real-IP
	// 4. X-Forwarded-For (first valid)
	// 5. gin's ClientIP fallback

	try := func(val string) (string, bool) {
		if val == "" {
			return "", false
		}
		v := strings.TrimSpace(val)
		if v == "" {
			return "", false
		}
		if net.ParseIP(v) == nil {
			return "", false
		}
		return v, true
	}

	if ip, ok := try(c.Request.Header.Get("CF-Connecting-IP")); ok { // Cloudflare official header
		return ip
	}
	if ip, ok := try(c.Request.Header.Get("True-Client-IP")); ok { // Some proxies / CDN
		return ip
	}
	if ip, ok := try(c.Request.Header.Get("X-Real-IP")); ok {
		return ip
	}

	// X-Forwarded-For: take the first valid public-looking IP (skip empties)
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, p := range parts {
			candidate := strings.TrimSpace(p)
			if candidate == "" {
				continue
			}
			if net.ParseIP(candidate) != nil {
				return candidate
			}
		}
	}

	return c.ClientIP()
}

// Local lightweight JSON marshal to avoid pulling in extra libs.
func jsonMarshal(v interface{}) ([]byte, error) { return json.Marshal(v) }
