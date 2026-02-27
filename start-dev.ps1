<#
  start-dev.ps1
  One-click dev runner for go-zero-trust-shop on Windows using Podman Machine.

  What it does:
    1) Starts Podman Machine if needed
    2) Brings up Vault + Postgres via podman-compose (inside the machine)
    3) Waits for ports (Vault 8200, Postgres 55432)
    4) Ensures /vault/bootstrap.sh exists, then runs it
    5) Prints fresh ROLE_ID / SECRET_ID
    6) Exports env vars and runs auth-svc (go run .)

  Usage:
    PowerShell → Set-ExecutionPolicy -Scope Process Bypass -Force
    & "$env:USERPROFILE\Documents\go-zero-trust-shop\start-dev.ps1"
#>

[CmdletBinding()]
param(
  [switch]$SkipRun # If supplied, will skip starting the Go service (will still bootstrap and print Role/Secret)
)

$ErrorActionPreference = "Stop"

# --- Settings / Paths ---
$root   = Join-Path $env:USERPROFILE "Documents\go-zero-trust-shop"
$deploy = Join-Path $root "deploy"
$svc    = Join-Path $root "services\auth-svc"

# Use Podman Machine ROOT connection for compose + exec
$connection = "podman-machine-default-root"
$composeProvider = "podman-compose" # You installed via: pip install podman-compose

function Wait-Port($HostName, $PortNumber, $Label) {
  Write-Host "⏳ Waiting for $Label on $($HostName):$PortNumber ..." -NoNewline
  $ok = $false
  for ($i=1; $i -le 180; $i++) { # up to ~90s
    $test = Test-NetConnection -ComputerName $HostName -Port $PortNumber -WarningAction SilentlyContinue
    if ($test.TcpTestSucceeded) { $ok = $true; break }
    Start-Sleep -Milliseconds 500
    if ($i % 10 -eq 0) {
      Write-Host -NoNewline " (still waiting)"
    } else {
      Write-Host -NoNewline "."
    }
  }
  Write-Host ""
  if (-not $ok) {
    throw "Timeout waiting for $Label on $($HostName):$PortNumber"
  }
}

# --- 1) Start Podman Machine (if needed) ---
Write-Host "▶ Ensuring Podman Machine is running..." -ForegroundColor Cyan
try { podman machine start | Out-Null } catch {}
# (If already running, the above is a no-op)

# --- 2) Bring the stack up (compose inside machine) ---
Write-Host "▶ Bringing stack up with podman compose (inside machine)..." -ForegroundColor Cyan
Set-Location $deploy
$env:PODMAN_COMPOSE_PROVIDER = $composeProvider

# Bring up (create or reuse)
podman --connection $connection compose up -d

Write-Host "`n▶ Current containers:"
podman --connection $connection ps

# --- 3) Wait for Vault & Postgres ports ---
Wait-Port 127.0.0.1 8200  "Vault HTTP"
Wait-Port 127.0.0.1 55432 "Postgres (host 55432 -> container 5432)"

# --- 4) Ensure /vault/bootstrap.sh exists; stream content via stdin ---
$bootstrap = @'
#!/bin/sh
set -e

export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root

echo "==> Enabling AppRole and Database engines (idempotent)"
vault auth enable approle 2>/dev/null || true
vault secrets enable database 2>/dev/null || true

echo "==> Configuring Postgres connection (Vault -> pg:5432, DB=shop)"
vault write database/config/shop-postgres \
  plugin_name="postgresql-database-plugin" \
  allowed_roles="app-role" \
  connection_url="postgresql://{{username}}:{{password}}@pg:5432/shop?sslmode=disable" \
  username="postgres" \
  password="postgres" >/dev/null

echo "==> (Re)writing dynamic DB role"
cat >/tmp/app-role.sql <<'SQL'
CREATE ROLE "{{name}}" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';
GRANT CONNECT ON DATABASE shop TO "{{name}}";
GRANT USAGE ON SCHEMA public TO "{{name}}";
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "{{name}}";
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO "{{name}}";
SQL

vault write database/roles/app-role \
  db_name="shop-postgres" \
  creation_statements=@/tmp/app-role.sql \
  default_ttl="1h" \
  max_ttl="24h" >/dev/null

echo "==> Writing app-policy"
cat >/tmp/app-policy.hcl <<'EOF'
path "secret/data/auth-svc/*" { capabilities = ["read"] }
path "database/creds/app-role" { capabilities = ["read"] }
EOF
vault policy write app-policy /tmp/app-policy.hcl >/dev/null

echo "==> Creating/Updating AppRole: auth-svc"
vault write auth/approle/role/auth-svc \
  token_policies="app-policy" \
  secret_id_num_uses=0 \
  token_ttl=60m \
  token_max_ttl=24h >/dev/null

ROLE_ID=$(vault read -field=role_id auth/approle/role/auth-svc/role-id)
SECRET_ID=$(vault write -f -field=secret_id auth/approle/role/auth-svc/secret-id)

echo ""
echo "==================== BOOTSTRAP OUTPUT ===================="
echo "ROLE_ID=$ROLE_ID"
echo "SECRET_ID=$SECRET_ID"
echo "=========================================================="
echo ""
'@

Write-Host "▶ Writing /vault/bootstrap.sh into the vault container..." -ForegroundColor Cyan
$bootstrap | podman --connection $connection exec -i vault sh -lc "cat > /vault/bootstrap.sh"
podman --connection $connection exec -it vault sh -lc "chmod +x /vault/bootstrap.sh && ls -l /vault/bootstrap.sh"

# --- 5) Run bootstrap & parse Role/Secret ---
Write-Host "▶ Running /vault/bootstrap.sh ..." -ForegroundColor Cyan
# Capture output (no TTY) for reliable parsing
$bootOut = podman --connection $connection exec vault sh /vault/bootstrap.sh
$bootOut | ForEach-Object { $_ } | Write-Host

# Parse ROLE_ID / SECRET_ID
$ROLE_ID   = ($bootOut | Select-String -Pattern '^ROLE_ID=(.+)$'   -AllMatches).Matches | Select-Object -Last 1 | ForEach-Object { $_.Groups[1].Value }
$SECRET_ID = ($bootOut | Select-String -Pattern '^SECRET_ID=(.+)$' -AllMatches).Matches | Select-Object -Last 1 | ForEach-Object { $_.Groups[1].Value }

if (-not $ROLE_ID -or -not $SECRET_ID) {
  throw "Could not parse ROLE_ID/SECRET_ID from bootstrap output."
}

Write-Host ""
Write-Host "✔ ROLE_ID   = $ROLE_ID" -ForegroundColor Green
Write-Host "✔ SECRET_ID = $SECRET_ID" -ForegroundColor Green

# --- 6) If not skipping, export env vars and run the Go service ---
if (-not $SkipRun) {
  Write-Host "`n▶ Starting auth-svc with fresh Role/Secret (Ctrl+C to stop) ..." -ForegroundColor Cyan

  # Clear libpq overrides that could force fallback to localhost:5432
  Get-ChildItem Env:PG* -ErrorAction SilentlyContinue | ForEach-Object {
    Remove-Item ("Env:{0}" -f $_.Name) -ErrorAction SilentlyContinue
  }

  Set-Location $svc

  $env:PORT            = "8083"
  $env:VAULT_ADDR      = "http://127.0.0.1:8200"
  $env:VAULT_ROLE_ID   = $ROLE_ID
  $env:VAULT_SECRET_ID = $SECRET_ID
  $env:DB_HOST         = "127.0.0.1"
  $env:DB_PORT         = "55432"

  Write-Host "  PORT=$($env:PORT)"
  Write-Host "  VAULT_ADDR=$($env:VAULT_ADDR)"
  Write-Host "  VAULT_ROLE_ID=$($env:VAULT_ROLE_ID)"
  Write-Host "  VAULT_SECRET_ID=$($env:VAULT_SECRET_ID)"
  Write-Host "  DB_HOST=$($env:DB_HOST)"
  Write-Host "  DB_PORT=$($env:DB_PORT)"

  go clean -cache -modcache
  go run .
}
else {
  Write-Host "`n(You passed -SkipRun) Use these to run the service manually:" -ForegroundColor Yellow
  Write-Host "  cd `"$svc`""
  Write-Host "  `$env:PORT='8083'"
  Write-Host "  `$env:VAULT_ADDR='http://127.0.0.1:8200'"
  Write-Host "  `$env:VAULT_ROLE_ID='$ROLE_ID'"
  Write-Host "  `$env:VAULT_SECRET_ID='$SECRET_ID'"
  Write-Host "  `$env:DB_HOST='127.0.0.1'"
  Write-Host "  `$env:DB_PORT='55432'"
  Write-Host "  go run ."
}