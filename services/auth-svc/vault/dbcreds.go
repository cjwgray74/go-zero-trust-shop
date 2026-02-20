
package vault

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type DBResp struct {
    Data struct {
        Username string `json:"username"`
        Password string `json:"password"`
    } `json:"data"`
}

func GetDynamicDBCreds(vaultAddr, token, role string) (string, string, error) {
    req, _ := http.NewRequest("GET", fmt.Sprintf("%s/v1/database/creds/%s", vaultAddr, role), nil)
    req.Header.Set("X-Vault-Token", token)
    resp, err := http.DefaultClient.Do(req)
    if err != nil { return "", "", err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return "", "", fmt.Errorf("db creds request failed: %s", resp.Status) }
    var out DBResp
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return "", "", err }
    return out.Data.Username, out.Data.Password, nil
}
