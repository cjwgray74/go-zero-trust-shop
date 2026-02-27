# Go Zero‑Trust Shop — Vault AppRole + Dynamic PostgreSQL (Podman Machine on Windows)

A production‑style **Zero‑Trust** demo service written in Go that:
- 🔐 Authenticates to **HashiCorp Vault** using **AppRole**
- 🗄️ Mints **short‑lived Postgres users** on demand via Vault’s **Database Secrets Engine**
- ⚙️ Connects using `pgx` with a robust `ConnectConfig` + short retry to absorb “just‑created role” races
- 🐳 Runs locally with **Podman Machine** (Windows‑friendly localhost forwarding)
- 🧪 Includes an end‑to‑end `/db/ping` that proves: AppRole → dynamic creds → Postgres connection → query

> **Why it matters:** No secrets in code. No static DB users. Each app run gets a new DB identity, automatically rotated by Vault.

---

## ✨ Highlights

- **Zero‑Trust pattern**: App → AppRole → Vault token → dynamic DB creds (username/password) → Postgres
- **Shortest happy path**: One script to start **Podman Machine**, bring up **Vault + Postgres**, bootstrap **AppRole + DB role**, and run the **Go service**
- **Windows‑friendly**: Uses **Podman Machine** so ports map to `127.0.0.1` (no WSL hacks)
- **Resilient DB connect**: tiny exponential backoff for brand‑new roles
- **Clean dev loop**: re‑bootstrap Vault dev mode on each restart; single place to copy fresh RoleID/SecretID

---

## 🏗️ Architecture

```mermaid
flowchart LR
  Dev[Developer on Windows] -->|F5 / start-dev.ps1| App[Go auth-svc :8083]
  subgraph Local Containers (Podman Machine)
    direction TB
    Vault[Vault (dev)\n:8200] --->|Database Secrets Engine\nread /database/creds/app-role| PG[(Postgres\n:5432)]
  end
  App -->|AppRole login\nrole_id + secret_id| Vault
  App -->|Dynamic DB creds\nusername/password (TTL)| PG
  App -->|GET /db/ping| PG
  App -->|GET /healthz| App
``