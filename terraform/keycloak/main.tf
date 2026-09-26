terraform {
  required_version = ">= 1.3.0"

  required_providers {
    keycloak = {
      source  = "mrparkers/keycloak"
      version = "~> 4.0"
    }
  }
}

provider "keycloak" {
  client_id = "admin-cli"
  username  = var.keycloak_admin_user
  password  = var.keycloak_admin_password
  url       = var.keycloak_url
}

data "keycloak_realm" "home" {
  realm = "home"
}

# ==========================================
# OIDC Clients
# ==========================================

resource "keycloak_openid_client" "argocd" {
  realm_id              = data.keycloak_realm.home.id
  client_id             = "argocd"
  name                  = "ArgoCD"
  access_type           = "CONFIDENTIAL"
  standard_flow_enabled = true
  valid_redirect_uris   = ["https://argocd.internal.rayq.app/auth/callback"]
}

resource "keycloak_openid_client" "vault" {
  realm_id              = data.keycloak_realm.home.id
  client_id             = "vault"
  name                  = "Vault"
  access_type           = "CONFIDENTIAL"
  standard_flow_enabled = true
  valid_redirect_uris = [
    "https://vault.internal.rayq.app/ui/vault/auth/oidc/oidc/callback",
    "https://vault.internal.rayq.app/oidc/callback",
  ]
}

resource "keycloak_openid_client" "kommande" {
  realm_id              = data.keycloak_realm.home.id
  client_id             = "kommande"
  name                  = "Kommande"
  access_type           = "CONFIDENTIAL"
  standard_flow_enabled = true
  valid_redirect_uris   = ["https://kommande.internal.rayq.app/auth/callback"]
}

resource "keycloak_openid_client" "games" {
  realm_id              = data.keycloak_realm.home.id
  client_id             = "games"
  name                  = "Games"
  access_type           = "CONFIDENTIAL"
  standard_flow_enabled = true
  valid_redirect_uris   = ["https://games.internal.rayq.app/api/auth/oidc/callback"]
}

resource "keycloak_openid_client" "dinks" {
  realm_id              = data.keycloak_realm.home.id
  client_id             = "dinks-web"
  name                  = "Dinks"
  access_type           = "CONFIDENTIAL"
  standard_flow_enabled = true
  valid_redirect_uris   = ["https://dinks.internal.rayq.app/auth/callback"]
}

# ==========================================
# Groups
# ==========================================

resource "keycloak_group" "argocd_admins" {
  realm_id = data.keycloak_realm.home.id
  name     = "argocd-admins"
}

resource "keycloak_group" "vault_admins" {
  realm_id = data.keycloak_realm.home.id
  name     = "vault-admins"
}

resource "keycloak_group" "kommande_admins" {
  realm_id = data.keycloak_realm.home.id
  name     = "kommande-admins"
}

resource "keycloak_group" "games_admins" {
  realm_id = data.keycloak_realm.home.id
  name     = "games-admins"
}

# ==========================================
# Users
# ==========================================

locals {
  group_ids = {
    "argocd-admins"   = keycloak_group.argocd_admins.id
    "vault-admins"    = keycloak_group.vault_admins.id
    "kommande-admins" = keycloak_group.kommande_admins.id
    "games-admins"    = keycloak_group.games_admins.id
  }
}

resource "keycloak_user" "users" {
  for_each = var.users

  realm_id = data.keycloak_realm.home.id
  username = each.key
  enabled  = true
  email    = each.value.email

  initial_password {
    value     = each.value.password
    temporary = false
  }
}

resource "keycloak_user_groups" "user_groups" {
  for_each = { for k, v in var.users : k => v if length(v.groups) > 0 }

  realm_id  = data.keycloak_realm.home.id
  user_id   = keycloak_user.users[each.key].id
  group_ids = [for g in each.value.groups : local.group_ids[g]]
}

# ==========================================
# Groups claim mappers (adds "groups" to ID tokens)
# ==========================================

resource "keycloak_openid_group_membership_protocol_mapper" "argocd_groups" {
  realm_id   = data.keycloak_realm.home.id
  client_id  = keycloak_openid_client.argocd.id
  name       = "groups"
  claim_name = "groups"
  full_path  = false
}

resource "keycloak_openid_group_membership_protocol_mapper" "vault_groups" {
  realm_id   = data.keycloak_realm.home.id
  client_id  = keycloak_openid_client.vault.id
  name       = "groups"
  claim_name = "groups"
  full_path  = false
}

resource "keycloak_openid_group_membership_protocol_mapper" "kommande_groups" {
  realm_id   = data.keycloak_realm.home.id
  client_id  = keycloak_openid_client.kommande.id
  name       = "groups"
  claim_name = "groups"
  full_path  = false
}

resource "keycloak_openid_group_membership_protocol_mapper" "games_groups" {
  realm_id   = data.keycloak_realm.home.id
  client_id  = keycloak_openid_client.games.id
  name       = "groups"
  claim_name = "groups"
  full_path  = false
}
