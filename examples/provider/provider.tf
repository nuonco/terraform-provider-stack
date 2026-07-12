terraform {
  required_providers {
    stack = {
      source = "nuonco/stack"
    }
  }
}

provider "stack" {
  # Base URL of the Nuon API. Defaults to https://api.nuon.co; override for
  # stage or BYOC control planes.
  # api_url = "https://api.nuon.co"
}
