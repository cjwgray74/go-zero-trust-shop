
# go-zero-trust-shop

A portfolio project demonstrating a Go microservice secured with **HashiCorp Vault** (AppRole + KV v2 + Database Secrets Engine for **dynamic PostgreSQL credentials**) and **HashiCorp Boundary** for zero‑trust access. Includes integration tests with **Testcontainers‑Go** and optional **OpenTelemetry** stubs.

## Quick start (Podman users)

```bash
cd deploy
podman compose up -d
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root
```

Then configure Vault (see below), fetch `VAULT_ROLE_ID` & `VAULT_SECRET_ID`, and run the service:

```bash
cd ../services/auth-svc
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_ROLE_ID=<role_id>
export VAULT_SECRET_ID=<secret_id>
go run .
```
