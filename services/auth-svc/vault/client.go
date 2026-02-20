
package vault

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

type AppRoleLoginResponse struct {
    Auth struct {
        ClientToken string `json:"client_token"`
    } `json:"auth"`
}

func LoginWithAppRole(vaultAddr, roleID, secretID string) (string, error) {
    payload := map[string]string{"role_id": roleID, "secret_id": secretID}
    body, _ := json.Marshal(payload)
    resp, err := http.Post(fmt.Sprintf("%s/v1/auth/approle/login", vaultAddr), "application/json", bytes.NewReader(body))
    if err != nil { return "", err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return "", fmt.Errorf("approle login failed: %s", resp.Status) }
    var out AppRoleLoginResponse
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return "", err }
    return out.Auth.ClientToken, nil
}

func EnvOrDie(key string) string {
    v := os.Getenv(key)
    if v == "" { panic("missing env: " + key) }
    return v
}
