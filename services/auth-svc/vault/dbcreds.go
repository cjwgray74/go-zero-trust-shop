package vault

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type dbCredsPayload struct {
	Data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"data"`
}

// GetDynamicDBCreds retrieves a fresh short-lived PostgreSQL username/password from Vault’s
// Database Secrets Engine for the given role. It adds small retries for transient 5xx errors
// and returns detailed error text (including the response body) for easier troubleshooting.
func GetDynamicDBCreds(vaultAddr, token, role string) (string, string, error) {
	// Normalize base URL (avoid double slashes)
	base := strings.TrimRight(vaultAddr, "/")
	url := fmt.Sprintf("%s/v1/database/creds/%s", base, role)

	client := &http.Client{Timeout: 10 * time.Second}

	const maxAttempts = 5
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("X-Vault-Token", token)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("vault creds request: %w", err)
			// Backoff and retry on network errors
			if attempt < maxAttempts {
				time.Sleep(backoff(attempt))
				continue
			}
			return "", "", lastErr
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var out dbCredsPayload
			if err := json.Unmarshal(body, &out); err != nil {
				return "", "", fmt.Errorf("vault creds decode: %w (body=%s)", err, string(body))
			}
			if out.Data.Username == "" || out.Data.Password == "" {
				return "", "", fmt.Errorf("vault creds missing fields (body=%s)", string(body))
			}
			return out.Data.Username, out.Data.Password, nil
		}

		// Retry on 5xx from Vault (transient)
		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
			lastErr = fmt.Errorf("vault 5xx getting db creds: %s body=%s", resp.Status, string(body))
			if attempt < maxAttempts {
				time.Sleep(backoff(attempt))
				continue
			}
			return "", "", lastErr
		}

		// Non-retryable client errors (e.g., 400/403) — include body to make policy/role issues obvious
		return "", "", fmt.Errorf("db creds request failed: %s body=%s", resp.Status, string(body))
	}

	// Should not reach here
	return "", "", fmt.Errorf("vault creds request exhausted retries: %v", lastErr)
}

func backoff(attempt int) time.Duration {
	// 100ms, 200ms, 400ms, 800ms...
	return time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond
}
