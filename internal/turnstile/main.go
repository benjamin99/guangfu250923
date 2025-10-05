package turnstile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type TokenVerifier interface {
	Verify(token string) (bool, error)
}

type TokenVerifierImpl struct {
	secretKey string
	client    *http.Client
}

type verifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

type verifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	Action      string   `json:"action,omitempty"`
}

func NewTokenVerifier(secretKey string) TokenVerifier {
	return &TokenVerifierImpl{
		secretKey: secretKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (v *TokenVerifierImpl) Verify(token string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("token is empty")
	}

	reqBody := verifyRequest{
		Secret:   v.secretKey,
		Response: token,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := v.client.Post(turnstileVerifyURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Errorf("failed to send verification request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var verifyResp verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return verifyResp.Success, nil
}
