package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

// phoneHomeResourceModel is the Terraform shape for the stack_phone_home
// resource. The reported run outputs are carried opaquely as a JSON-encoded
// string in payload; the provider injects request_type (from the resource
// lifecycle), phone_home_type, and the resolved install inputs before POSTing.
type phoneHomeResourceModel struct {
	ID            types.String `tfsdk:"id"`
	InstallID     types.String `tfsdk:"install_id"`
	PhoneHomeURL  types.String `tfsdk:"phone_home_url"`
	PhoneHomeType types.String `tfsdk:"phone_home_type"`
	Payload       types.String `tfsdk:"payload"`
	Inputs        types.Map    `tfsdk:"inputs"`
}
