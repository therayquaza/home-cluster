terraform {
  required_version = ">= 1.3.0"

  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "~> 5.0"
    }
  }
}

provider "vault" {
  address         = var.vault_address
  token           = var.vault_root_token
  skip_tls_verify = true
}

# Enable KV v2 secrets engine if not already enabled
resource "vault_mount" "kv" {
  path        = "kv"
  type        = "kv"
  options     = { version = "2" }
  description = "KV Version 2 secret engine mount"
}

# ==========================================
# Cloudflare Secrets
# ==========================================

resource "vault_kv_secret_v2" "cloudflare" {
  mount = vault_mount.kv.path
  name  = "cloudflare"

  data_json = jsonencode({
    CLOUDFLARE_API_TOKEN = var.cloudflare_api_token
    email                = var.cloudflare_email
  })
}

# ==========================================
# Keycloak Secrets
# ==========================================
# Run twice:
#   1st apply (bootstrap): provide admin-password + postgresql-password only
#   2nd apply (post-realm): add client secrets from `terraform output` in terraform/keycloak/

resource "vault_kv_secret_v2" "keycloak" {
  mount = vault_mount.kv.path
  name  = "keycloak"

  data_json = jsonencode({
    admin-password         = var.keycloak_admin_password
    postgresql-password    = var.keycloak_postgresql_password
    argocd-client-secret   = var.keycloak_argocd_client_secret
    vault-client-secret    = var.keycloak_vault_client_secret
    kommande-client-secret = var.keycloak_kommande_client_secret
    games-client-secret    = var.keycloak_games_client_secret
    dinks-web-client-secret = var.keycloak_dinks_client_secret
  })
}

# ==========================================
# Dinks Secrets
# ==========================================

resource "vault_kv_secret_v2" "dinks" {
  mount = vault_mount.kv.path
  name  = "dinks"

  data_json = jsonencode({
    db-password = var.dinks_db_password
    mongo-uri   = "mongodb://dinks:${var.dinks_db_password}@mongo.dinks.svc.cluster.local:27017/dinks?authSource=admin"
    jwt-secret  = var.dinks_jwt_secret
  })
}

# ==========================================
# GitHub Runner Secrets
# ==========================================

resource "vault_kv_secret_v2" "github" {
  mount = vault_mount.kv.path
  name  = "github"

  data_json = jsonencode({
    pat             = var.github_runner_pat
    pat-expiry      = var.github_runner_pat_expiry
    discord-webhook = var.discord_webhook_url
  })
}
