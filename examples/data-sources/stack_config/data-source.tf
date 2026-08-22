# Read an install stack's rendered configuration from the Nuon control plane. The
# install ID is not a secret — the provider's credentials are what authorize the
# read. See the repository's examples/data-source-gcp for a full module wiring.
data "stack_config" "this" {
  install_id = var.install_id
}

output "install_id" {
  value = data.stack_config.this.install_id
}
