# Read an install stack's rendered configuration from the Nuon control plane,
# keyed by the phone_home_id supplied to the install-stacks module. See the
# repository's examples/data-source-gcp for a full end-to-end module wiring.
data "stack_config" "this" {
  phone_home_id = var.phone_home_id
}

output "install_id" {
  value = data.stack_config.this.install_id
}
