terraform {
  required_version = ">= 1.3.0"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "5.25.0"
    }
  }
}

provider "cloudflare" {
  #email   = var.cloudflare_email
  api_token = var.cloudflare_api_key
}

resource "cloudflare_record" "multi_record" {
  for_each = var.dns_records

  zone_id = var.zone_id
  name    = each.value.name
  content = each.value.content
  type    = each.value.type
  ttl     = each.value.ttl
  proxied = each.value.proxied
}
