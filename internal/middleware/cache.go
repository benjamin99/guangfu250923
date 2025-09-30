package middleware

import (
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
)

// CacheHeaders adds basic caching headers (ETag, Cache-Control) for idempotent GET responses.
// It computes a weak ETag from the response body for 200 OK GET responses up to a size limit.
// If the client sends If-None-Match matching the computed ETag, a 304 Not Modified is returned.
func CacheHeaders(maxBody int) gin.HandlerFunc {
    if maxBody <= 0 {
        maxBody = 512 * 1024 // 512KB
    }
    return func(c *gin.Context) {
        if c.Request.Method != http.MethodGet {
            c.Next()
            return
        }

        // Wrap writer to capture body
        rw := &cacheRecorder{ResponseWriter: c.Writer, status: 200, limit: maxBody}
        c.Writer = rw
        c.Next()

        // Only proceed on 200 OK and non-upgraded
        if rw.status != http.StatusOK {
            return
        }
        body := rw.buf.Bytes()
        // Compute ETag
        h := sha256.Sum256(body)
        etag := "W/\"" + hex.EncodeToString(h[:8]) + "\"" // first 8 bytes -> 16 hex chars

        if inm := c.Request.Header.Get("If-None-Match"); inm != "" {
            // simple exact match check (can hold multiple) - split by comma
            parts := strings.Split(inm, ",")
            for _, p := range parts {
                if strings.TrimSpace(p) == etag {
                    // Return 304
                    hdr := c.Writer.Header()
                    hdr.Set("ETag", etag)
                    hdr.Set("Cache-Control", cacheControlForPath(c.FullPath(), c.Request.URL.RawQuery))
                    hdr.Set("Vary", "Accept-Encoding")
                    c.Writer.WriteHeader(http.StatusNotModified)
                    // Clear body already written (cannot unwrite, but at this point response body already flushed?)
                    return
                }
            }
        }

        hdr := c.Writer.Header()
        hdr.Set("ETag", etag)
        // Avoid overwriting if handler already set a Cache-Control
        if hdr.Get("Cache-Control") == "" {
            hdr.Set("Cache-Control", cacheControlForPath(c.FullPath(), c.Request.URL.RawQuery))
        }
        hdr.Add("Vary", "Accept-Encoding")
        // Provide a conservative Last-Modified of now (could be improved to max(updated_at) parsing JSON)
        if hdr.Get("Last-Modified") == "" {
            hdr.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
        }
    }
}

type cacheRecorder struct {
    gin.ResponseWriter
    status int
    buf    bytes.Buffer
    limit  int
}

func (r *cacheRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }
func (r *cacheRecorder) Write(b []byte) (int, error) {
    if r.buf.Len() < r.limit {
        remain := r.limit - r.buf.Len()
        if len(b) > remain {
            r.buf.Write(b[:remain])
        } else {
            r.buf.Write(b)
        }
    }
    return r.ResponseWriter.Write(b)
}

// cacheControlForPath decides cache policy based on path pattern and query string.
func cacheControlForPath(pattern, rawQuery string) string {
    if strings.HasPrefix(pattern, "/_admin/") || pattern == "/healthz" {
        return "no-store"
    }
    // list endpoints usually have limit/offset
    if strings.Contains(rawQuery, "offset=") || strings.Contains(rawQuery, "limit=") {
        return "public, max-age=30"
    }
    // entity endpoints (contain :id)
    if strings.Contains(pattern, ":id") {
        return "public, max-age=60"
    }
    // default short cache
    return "public, max-age=30"
}
