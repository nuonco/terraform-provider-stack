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
}
