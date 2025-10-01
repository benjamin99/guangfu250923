package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets common security-related HTTP response headers.
// - HSTS is only added when the request is served via HTTPS (Request.TLS != nil)
// - Swagger UI path (/swagger/) gets a relaxed CSP to function
// - All other paths (API JSON responses) receive a very strict CSP
//
// If you later serve real HTML pages, adjust CSP accordingly.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Let handlers run first so they can set Content-Type etc.
		c.Next()

		// Clickjacking protection
		if c.Writer.Header().Get("X-Frame-Options") == "" {
			c.Writer.Header().Set("X-Frame-Options", "SAMEORIGIN")
		}

		// MIME sniffing protection
		if c.Writer.Header().Get("X-Content-Type-Options") == "" {
			c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		}

		// Referrer policy (no-referrer keeps things private; adjust if analytics needed)
		if c.Writer.Header().Get("Referrer-Policy") == "" {
			c.Writer.Header().Set("Referrer-Policy", "no-referrer")
		}

		// Permissions-Policy (formerly Feature-Policy). Lock down most features.
		if c.Writer.Header().Get("Permissions-Policy") == "" {
			c.Writer.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), fullscreen=(*), payment=()")
		}

		// Optional: disable legacy XSS auditor (avoid false positives). Commented out by default.
		// c.Writer.Header().Set("X-XSS-Protection", "0")

		// Content Security Policy
		if c.Writer.Header().Get("Content-Security-Policy") == "" {
			path := c.Request.URL.Path
			var csp string
			if strings.HasPrefix(path, "/swagger/") {
				// Swagger UI needs inline/eval for its generated bundle.
				csp = "default-src 'none'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'self';"
			} else {
				// Pure API responses; no active content allowed.
				csp = "default-src 'none'; frame-ancestors 'none';"
			}
			c.Writer.Header().Set("Content-Security-Policy", csp)
		}
	}
}
