variable "db_admin_username" {
  type        = string
  default     = "postgres"
  description = "Admin user for initial Vault DB connection"
}

variable "db_admin_password" {
  type        = string
  default     = "postgres"
  sensitive   = true
  description = "Admin password for initial Vault DB connection"
}

variable "db_host" {
  type        = string
  default     = "pg"
  description = "Hostname as seen by Vault (service name in compose network)"
}

variable "db_port" {
  type        = number
  default     = 5432
  description = "Container port for Postgres (5432 inside container)"
}

variable "db_name" {
  type        = string
  default     = "shop"
}

variable "vault_db_mount_path" {
  type        = string
  default     = "database"
}

variable "vault_role_name" {
  type        = string
  default     = "auth-svc"
  description = "AppRole name for the service"
}

variable "vault_policy_name" {
  type        = string
  default     = "app-policy"
}
