variable "vault_address" {
  description = "Vault server address"
  type        = string
  default     = "http://localhost:8200"
}

variable "vault_root_token" {
  description = "Vault root token"
  type        = string
  sensitive   = true
}


# Cloudflare variables
variable "cloudflare_api_token" {
  description = "Cloudflare API token"
  type        = string
  sensitive   = true
}

variable "cloudflare_email" {
  description = "Cloudflare account email"
  type        = string
  sensitive   = true
}

# Keycloak variables — see terraform/keycloak/ for client secret values

variable "keycloak_admin_password" {
  description = "Keycloak admin password (set before first deploy)"
  type        = string
  sensitive   = true
}

variable "keycloak_postgresql_password" {
  description = "PostgreSQL password for Keycloak (set before first deploy)"
  type        = string
  sensitive   = true
}

variable "keycloak_argocd_client_secret" {
  description = "OIDC client secret for ArgoCD (from terraform/keycloak output)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "keycloak_vault_client_secret" {
  description = "OIDC client secret for Vault (from terraform/keycloak output)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "keycloak_kommande_client_secret" {
  description = "OIDC client secret for Kommande (from terraform/keycloak output)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "keycloak_games_client_secret" {
  description = "OIDC client secret for Games (from terraform/keycloak output)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "keycloak_dinks_client_secret" {
  description = "OIDC client secret for Dinks (from terraform/keycloak output)"
  type        = string
  sensitive   = true
  default     = ""
}

# Dinks variables

variable "dinks_db_password" {
  description = "MongoDB root password for Dinks"
  type        = string
  sensitive   = true
}

variable "dinks_jwt_secret" {
  description = "JWT signing secret for Dinks backend"
  type        = string
  sensitive   = true
}

# GitHub Runner variables

variable "github_runner_pat" {
  description = "GitHub classic PAT for actions-runner-controller (scope: admin:org)"
  type        = string
  sensitive   = true
}

variable "github_runner_pat_expiry" {
  description = "Expiry date of the GitHub PAT (YYYY-MM-DD) — update on each rotation"
  type        = string
  sensitive   = true
}

variable "discord_webhook_url" {
  description = "Discord webhook URL for PAT expiry notifications"
  type        = string
  sensitive   = true
}

