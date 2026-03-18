# VAULT_ADDR and VAULT_TOKEN come from your environment.
provider "vault" {
  # nothing required if env vars are set
}

# Optional: only needed if you want Terraform to manage DB schema or roles directly in Postgres.
# Your app already creates tables via migrations, so you can skip this.
provider "postgresql" {
  host            = "127.0.0.1"
  port            = 55432
  database        = "shop"
  username        = "postgres"
  password        = "postgres"
  sslmode         = "disable"
  connect_timeout = 5
}

# Optional: only if you want Terraform to manage Vault/PG containers via Docker API exposed by Podman Machine.
# provider "docker" { host = "npipe:////./pipe/docker_engine" }
