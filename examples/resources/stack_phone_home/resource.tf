# Report the result of an install-stack run to the Nuon control plane. The
# resource lifecycle drives the phone-home request_type: Create on first apply,
# Update when the payload changes, Delete on destroy.
resource "stack_phone_home" "this" {
  install_id      = data.stack_config.this.install_id
  phone_home_id   = var.phone_home_id
  phone_home_type = "gcp"

  payload = jsonencode({
    network_name   = module.stack.network_name
    install_inputs = data.stack_config.this.install_inputs
  })
}
