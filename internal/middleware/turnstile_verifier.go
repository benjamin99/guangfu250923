package middleware

import (
	"net/http"
	"os"
	"strings"

	"guangfu250923/internal/turnstile"

	"github.com/gin-gonic/gin"
)

type tokenRequest struct {
	CFTurnstileResponse string `json:"cf-turnstile-response"`
}

func setupVerifier() turnstile.TokenVerifier {
	secretKey := os.Getenv("TURNSTILE_SECRET_KEY")
	if !strings.EqualFold(os.Getenv("VERIFY_TURNSTILE"), "true") || secretKey == "" {
		return nil
	}

	return turnstile.NewTokenVerifier(turnstile.NewTokenVerifierOptions{
		APIURL:    os.Getenv("TURNSTILE_API_URL"),
		SecretKey: secretKey,
	})
}

func TurnstileVerifier() gin.HandlerFunc {
	verifier := setupVerifier()

	return func(c *gin.Context) {
		// if verifier is not setup, just proceed
		if verifier == nil {
			c.Next()
			return
		}

		var in tokenRequest
		if err := c.ShouldBindBodyWithJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		token := in.CFTurnstileResponse
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "blocked", "reason": "turnstile token is required"})
			c.Abort()
			return
		}

		success, err := verifier.Verify(turnstile.VerifyOptions{
			Token: token,
			// TODO: should extract the clientIP function to some utils package
			RemoteIP: c.ClientIP(),
		})

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "blocked", "reason": "failed to verify turnstile token"})
			c.Abort()
			return
		}

		if !success {
			c.JSON(http.StatusBadRequest, gin.H{"error": "blocked", "reason": "invalid turnstile token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
