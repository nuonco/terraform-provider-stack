# Report the result of an install-stack run to the Nuon control plane. The
# resource lifecycle drives the phone-home request_type: Create on first apply,
# Update when the payload changes, Delete on destroy.
#
# phone_home_url comes from the config data source rather than a variable: it
# embeds a per-stack-version identifier the caller has no other way to know.
resource "stack_phone_home" "this" {
  install_id      = data.stack_config.this.install_id
  phone_home_url  = data.stack_config.this.phone_home_url
  phone_home_type = "gcp"

  payload = jsonencode({
    network_name   = module.stack.network_name
    install_inputs = data.stack_config.this.install_inputs
  })
}
