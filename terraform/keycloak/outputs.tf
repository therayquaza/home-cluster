output "argocd_client_secret" {
  value     = keycloak_openid_client.argocd.client_secret
  sensitive = true
}

output "vault_client_secret" {
  value     = keycloak_openid_client.vault.client_secret
  sensitive = true
}

output "kommande_client_secret" {
  value     = keycloak_openid_client.kommande.client_secret
  sensitive = true
}

output "games_client_secret" {
  value     = keycloak_openid_client.games.client_secret
  sensitive = true
}

output "dinks_client_secret" {
  value     = keycloak_openid_client.dinks.client_secret
  sensitive = true
}
