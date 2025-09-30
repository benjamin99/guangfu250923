package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CacheHeaders adds basic caching headers (ETag, Cache-Control) for idempotent GET responses.
// It computes a weak ETag from the response body for 200 OK GET responses up to a size limit.
// If the client sends If-None-Match matching the computed ETag, a 304 Not Modified is returned.
func CacheHeaders(maxBody int) gin.HandlerFunc {
	if maxBody <= 0 {
		maxBody = 512 * 1024
	}
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		rw := &cacheRecorder{ResponseWriter: c.Writer, status: 200, limit: maxBody}
		c.Writer = rw
		c.Next()
		// if handler already wrote a redirect or error, just flush as-is
		if rw.status != http.StatusOK {
			// write through
			writeBuffered(rw)
			return
		}
		body := rw.buf.Bytes()
		h := sha256.Sum256(body)
		etag := "W/\"" + hex.EncodeToString(h[:8]) + "\""
		hdr := rw.Header()
		// Handle conditional
		if inm := c.Request.Header.Get("If-None-Match"); inm != "" {
			parts := strings.Split(inm, ",")
			for _, p := range parts {
				if strings.TrimSpace(p) == etag { // 304
					hdr.Del("Content-Length")
					hdr.Set("ETag", etag)
					if hdr.Get("Cache-Control") == "" {
						hdr.Set("Cache-Control", cacheControlForPath(c.FullPath(), c.Request.URL.RawQuery))
					}
					hdr.Set("Vary", "Accept-Encoding")
					if hdr.Get("Last-Modified") == "" {
						hdr.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
					}
					rw.ResponseWriter.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}
		hdr.Set("ETag", etag)
		if hdr.Get("Cache-Control") == "" {
			hdr.Set("Cache-Control", cacheControlForPath(c.FullPath(), c.Request.URL.RawQuery))
		}
		hdr.Add("Vary", "Accept-Encoding")
		if hdr.Get("Last-Modified") == "" {
			hdr.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		}
		hdr.Set("Content-Length", strconv.Itoa(len(body)))
		// Now flush body (status might have been set by handler)
		if rw.headerWritten {
			// handler called WriteHeader; we need to replay status
			rw.ResponseWriter.WriteHeader(rw.status)
		}
		if len(body) > 0 {
			rw.ResponseWriter.Write(body)
		}
	}
}

type cacheRecorder struct {
	gin.ResponseWriter
	status        int
	buf           bytes.Buffer
	limit         int
	headerWritten bool
}

func (r *cacheRecorder) WriteHeader(code int) {
	r.status = code
	r.headerWritten = true /* defer real write until flush */
}
func (r *cacheRecorder) Write(b []byte) (int, error) {
	if r.buf.Len() < r.limit {
		remain := r.limit - r.buf.Len()
		if len(b) > remain {
			r.buf.Write(b[:remain])
		} else {
			r.buf.Write(b)
		}
	}
	// swallow for now
	return len(b), nil
}

func writeBuffered(r *cacheRecorder) {
	if r.headerWritten {
		r.ResponseWriter.WriteHeader(r.status)
	}
	if r.buf.Len() > 0 {
		r.ResponseWriter.Write(r.buf.Bytes())
	}
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
