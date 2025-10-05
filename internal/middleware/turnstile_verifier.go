package middleware

import (
	"net/http"
	"os"

	"guangfu250923/internal/turnstile"

	"github.com/gin-gonic/gin"
)

func TurnstileVerifier() gin.HandlerFunc {
	verifier := turnstile.NewTokenVerifier(os.Getenv("TURNSTILE_SECRET_KEY"))

	return func(c *gin.Context) {
		if os.Getenv("VERIFY_TURNSTILE") == "false" {
			c.Next()
			return
		}

		token := c.GetHeader("X-ReCaptcha-Token")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ReCaptcha token is required"})
			c.Abort()
			return
		}

		success, err := verifier.Verify(token)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to verify token" + err.Error()})
			c.Abort()
			return
		}

		if !success {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
