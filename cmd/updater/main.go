package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Simple response wrapper
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// performUpdate downloads latest release asset and restarts systemd service.
func performUpdate(ctx context.Context, repo, assetPattern, service, installPath string, dryRun bool) (map[string]any, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github api status %d", resp.StatusCode)
	}
	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.TagName == "" {
		return nil, errors.New("missing tag_name in release")
	}
	var assetURL, assetName string
	for _, a := range payload.Assets {
		if a.Name == "" || a.URL == "" {
			continue
		}
		if assetPattern == "" || strings.Contains(a.Name, assetPattern) {
			assetURL = a.URL
			assetName = a.Name
			break
		}
	}
	if assetURL == "" && len(payload.Assets) > 0 { // fallback first
		assetURL = payload.Assets[0].URL
		assetName = payload.Assets[0].Name
	}
	if assetURL == "" {
		return nil, errors.New("no asset matched pattern")
	}

	tmpDir, err := os.MkdirTemp("", "updater-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	newBin := filepath.Join(tmpDir, "new-bin")

	if dryRun {
		return map[string]any{"tag": payload.TagName, "asset": assetName, "dry_run": true}, nil
	}

	ar, err := client.Get(assetURL)
	if err != nil {
		return nil, err
	}
	defer ar.Body.Close()
	if ar.StatusCode != 200 {
		return nil, fmt.Errorf("asset download status %d", ar.StatusCode)
	}
	f, err := os.Create(newBin)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, ar.Body); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	if err := os.Chmod(newBin, 0o755); err != nil {
		return nil, err
	}

	// optional simple integrity marker (hash)
	hashFile, _ := os.Open(newBin)
	sha := sha256.New()
	io.Copy(sha, hashFile)
	hashFile.Close()
	sum := hex.EncodeToString(sha.Sum(nil))

	backupPath := installPath + ".bak." + time.Now().Format("20060102150405")
	if _, err := os.Stat(installPath); err == nil {
		_ = copyFile(installPath, backupPath)
	}
	if err := os.Rename(newBin, installPath); err != nil {
		return nil, err
	}

	if out, err := runCmd("systemctl", "stop", service); err != nil {
		return nil, fmt.Errorf("stop failed: %v / %s", err, out)
	}
	if out, err := runCmd("systemctl", "start", service); err != nil {
		return nil, fmt.Errorf("start failed: %v / %s", err, out)
	}
	// quick health check (optional) can be added here

	return map[string]any{
		"tag":     payload.TagName,
		"asset":   assetName,
		"backup":  backupPath,
		"sha256":  sum,
		"service": service,
	}, nil
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func main() {
	addr := envOr("UPDATER_LISTEN", ":9090")
	apiKey := os.Getenv("UPDATE_API_KEY")
	if apiKey == "" {
		fmt.Println("[WARN] UPDATE_API_KEY not set; all requests will be rejected")
	}
	service := envOr("SERVICE_NAME", "guangfu250923")
	install := envOr("INSTALL_PATH", "/etc/guangfu250923/guangfu250923")
	repo := envOr("GITHUB_REPO", "PichuChen/guangfu250923")
	pattern := os.Getenv("ASSET_PATTERN") // optional

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })

	http.HandleFunc("/upgrade-service", func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("key")
		}
		if apiKey == "" || key != apiKey {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		dryRun := r.URL.Query().Get("dry_run") == "true"
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		res, err := performUpdate(ctx, repo, pattern, service, install, dryRun)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	fmt.Printf("[updater] listening on %s (service=%s repo=%s)\n", addr, service, repo)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "listen error: %v\n", err)
		os.Exit(1)
	}
}
