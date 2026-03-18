output "VAULT_ROLE_ID" {
  value       = data.vault_approle_auth_backend_role_id.svc.role_id
  description = "Use this for VAULT_ROLE_ID"
}

output "VAULT_SECRET_ID" {
  value       = vault_approle_auth_backend_role_secret_id.svc.secret_id
  sensitive   = true
  description = "Use this for VAULT_SECRET_ID"
}