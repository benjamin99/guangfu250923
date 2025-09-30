package middleware

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// responseRecorder captures status and body (if needed truncated) for logging.
type responseRecorder struct {
	gin.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
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
		go func(method, path, rawQuery, ip string, status int, errText string, headers []byte, took time.Duration) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = pool.Exec(ctx, `insert into request_logs(method,path,query,ip,headers,status_code,error,duration_ms) values($1,$2,$3,$4,$5::jsonb,$6,$7,$8)`,
				method, path, rawQuery, ip, string(headers), status, nullIfEmpty(errText), int(took.Milliseconds()))
		}(c.Request.Method, c.FullPath(), c.Request.URL.RawQuery, clientIP(c), recorder.status, errMsg, headersJSON, dur)
	}
}

// Helper functions (minimal to avoid extra deps)

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
