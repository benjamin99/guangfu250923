package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const authValidateURL = "https://api.line.me/oauth2/v2.1/verify"

func validateToken(header string) bool {
	if len(header) < 7 || !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	return strings.TrimPrefix(header, "Bearer ") != ""
}

func AuthTokenVerifier() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if !validateToken(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		form := url.Values{}
		form.Set("access_token", string(decoded))
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, authValidateURL, bytes.NewBufferString(form.Encode()))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate token", "reason": err.Error()})
			c.Abort()
			return
		}
		defer resp.Body.Close()

		// the LINE api has the rate limiting, so we need to handle the 429 status code
		if resp.StatusCode == http.StatusTooManyRequests {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			c.Abort()
			return
		}

		if resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
