terraform {
  required_providers {
    stack = {
      source = "nuonco/stack"
    }
  }
}

provider "stack" {
  # Base URL of the Nuon runner API. Defaults to https://runner.nuon.co; override for
  # stage or BYOC control planes.
  # api_url = "https://runner.nuon.co"

  # API token issued by the vendor alongside the install. Falls back to
  # NUON_API_TOKEN.
  api_token = var.api_token
}

# Alternatively, authenticate with OIDC and store no secret at all. In GitHub
# Actions, grant `permissions: id-token: write` and set only the org:
#
# provider "stack" {
#   org_id = var.org_id
# }
