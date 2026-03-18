############################################
# Auth & Mounts
############################################

resource "vault_auth_backend" "approle" {
  type = "approle"
}

resource "vault_mount" "database" {
  path = "database"
  type = "database"
}

############################################
# DB Connection (Vault -> Postgres)
############################################

resource "vault_database_secret_backend_connection" "pg" {
  backend       = vault_mount.database.path
  name          = "shop-postgres"
  allowed_roles = ["app-role"]

  # Simplest for local dev: inline admin creds in DSN.
  postgresql {
    connection_url = "postgresql://postgres:postgres@pg:5432/shop?sslmode=disable"
  }

  # If you prefer templated DSN + data map, use this instead:
  # postgresql {
  #   connection_url = "postgresql://{{username}}:{{password}}@pg:5432/shop?sslmode=disable"
  # }
  # data = {
  #   username = var.db_admin_username
  #   password = var.db_admin_password
  # }
}

############################################
# Dynamic Role (add CREATE privilege for migrations)
############################################

locals {
  creation_sql = <<-SQL
    CREATE ROLE "{{name}}" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';
    GRANT CONNECT ON DATABASE shop TO "{{name}}";
    GRANT USAGE ON SCHEMA public TO "{{name}}";
    -- REQUIRED so migrations can create tables
    GRANT CREATE ON SCHEMA public TO "{{name}}";
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "{{name}}";
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO "{{name}}";
  SQL
}

resource "vault_database_secret_backend_role" "app_role" {
  backend             = vault_mount.database.path
  name                = "app-role"
  db_name             = vault_database_secret_backend_connection.pg.name
  creation_statements = [local.creation_sql]

  # <-- FIXED: use seconds as numbers, not strings
  default_ttl = 3600   # 1h
  max_ttl     = 86400  # 24h
}

############################################
# Policy allowing app to read needed paths
############################################

data "vault_policy_document" "app" {
  rule {
    path         = "secret/data/auth-svc/*"
    capabilities = ["read"]
  }
  rule {
    path         = "${vault_mount.database.path}/creds/app-role"
    capabilities = ["read"]
  }
}

resource "vault_policy" "app_policy" {
  name   = "app-policy"
  policy = data.vault_policy_document.app.hcl
}

############################################
# AppRole for the service
############################################

resource "vault_approle_auth_backend_role" "auth_svc" {
  backend        = vault_auth_backend.approle.path
  role_name      = "auth-svc"
  token_policies = [vault_policy.app_policy.name]

  # (already fixed earlier) seconds, not "60m"/"24h"
  secret_id_num_uses = 0
  token_ttl          = 3600   # 60m
  token_max_ttl      = 86400  # 24h
}

# Read RoleID (data source)
data "vault_approle_auth_backend_role_id" "svc" {
  backend   = vault_auth_backend.approle.path
  role_name = vault_approle_auth_backend_role.auth_svc.role_name
}

# Generate a fresh SecretID (resource)
resource "vault_approle_auth_backend_role_secret_id" "svc" {
  backend   = vault_auth_backend.approle.path
  role_name = vault_approle_auth_backend_role.auth_svc.role_name
}