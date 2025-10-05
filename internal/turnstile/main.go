package turnstile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type VerifyOptions struct {
	Token    string
	RemoteIP string
}
type TokenVerifier interface {
	Verify(opt VerifyOptions) (bool, error)
}

type TokenVerifierImpl struct {
	apiURL    string
	secretKey string
	client    *http.Client
}

type verifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
	RemoteIP string `json:"remoteip,omitempty"`
}

type verifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	Action      string   `json:"action,omitempty"`
}

type NewTokenVerifierOptions struct {
	APIURL    string
	SecretKey string
}

func NewTokenVerifier(opt NewTokenVerifierOptions) TokenVerifier {
	if opt.APIURL == "" {
		opt.APIURL = defaultTurnstileVerifyURL
	}

	return &TokenVerifierImpl{
		apiURL:    opt.APIURL,
		secretKey: opt.SecretKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (v *TokenVerifierImpl) Verify(opt VerifyOptions) (bool, error) {
	if opt.Token == "" {
		return false, fmt.Errorf("token is empty")
	}

	reqBody := verifyRequest{
		Secret:   v.secretKey,
		Response: opt.Token,
		RemoteIP: opt.RemoteIP,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := v.client.Post(v.apiURL, "application/json", bytes.NewBuffer(jsonData))
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
