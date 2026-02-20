
# Allow service to read app KV (optional usage)
path "secret/data/auth-svc/*" {
  capabilities = ["read"]
}

# Allow service to request dynamic DB credentials for role 'app-role'
path "database/creds/app-role" {
  capabilities = ["read"]
}
