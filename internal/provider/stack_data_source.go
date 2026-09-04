package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	stack "github.com/nuonco/nuon/sdks/stack"
)

var (
	_ datasource.DataSource              = (*stackDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*stackDataSource)(nil)
)

// stackDataSource is the stack_config data source. It reads an install stack's
// rendered configuration from the Nuon control plane.
type stackDataSource struct {
	cfg *providerConfig
}

// NewStackDataSource is the data source factory registered with the provider.
func NewStackDataSource() datasource.DataSource {
	return &stackDataSource{}
}

func (d *stackDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config"
}

func (d *stackDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cfg, ok := req.ProviderData.(*providerConfig)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("expected *providerConfig, got %T", req.ProviderData))
		return
	}
	d.cfg = cfg
}

func (d *stackDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	roleAttrs := map[string]schema.Attribute{
		"permissions":     schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IAM permissions bound to the role's service account."},
		"policies":        schema.MapAttribute{Computed: true, ElementType: types.ListType{ElemType: types.StringType}, MarkdownDescription: "Per-policy custom roles (policy name → permissions): one custom role per policy."},
		"predefined_role": schema.StringAttribute{Computed: true, MarkdownDescription: "Predefined role bound to the service account, if any."},
		"enabled":         schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the role should be created."},
	}

	awsRoleAttrs := map[string]schema.Attribute{
		"permissions":            schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IAM action strings granted via an inline policy."},
		"inline_policy_document": schema.StringAttribute{Computed: true, MarkdownDescription: "JSON IAM policy document attached as an inline policy. Takes precedence over permissions."},
		"managed_policy_arns":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs to attach to the role."},
		"enabled":                schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the role should be created."},
	}

	azureRoleAttrs := map[string]schema.Attribute{
		"actions":        schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Azure action strings granted via a custom role definition."},
		"built_in_roles": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Built-in role GUIDs assigned directly. Already resolved from names by the control plane."},
		"enabled":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the role should be created."},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Nuon install stack's rendered configuration (runner, permissions, inputs, secrets) from the control plane. Intended for use inside install-stacks modules so the config is read from the API rather than passed in as tfvars.",
		Attributes: map[string]schema.Attribute{
			"install_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Nuon install ID. Not a secret — the provider's credentials are what authorize this read.",
			},

			"org_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Nuon organization ID."},
			"app_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Nuon application ID."},
			"cloud":          schema.StringAttribute{Computed: true, MarkdownDescription: "Target cloud (`aws`, `azure`, or `gcp`)."},
			"runner_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Runner ID for this install."},
			"runner_api_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Runner API URL the runner reports to."},
			"phone_home_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Phone-home URL the module reports run completion to."},

			"install_inputs": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Customer install-input values.",
			},
			"required_input_names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				// Names, for the same reason sensitive_input_names is: install_inputs
				// carries no per-key metadata. The module uses these to fail the plan
				// when a required input resolves to an empty value.
				MarkdownDescription: "Names of the entries in `install_inputs` the app declares required.",
			},
			"sensitive_input_names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				// Names, not values: install_inputs is a released map[string]string,
				// and a terraform map can only be sensitive as a whole. Marking the
				// whole map sensitive would redact every non-sensitive input too, so
				// the names are surfaced and the module decides what to do with them.
				MarkdownDescription: "Names of the entries in `install_inputs` the app declares sensitive.",
			},
			"auto_generate_secrets": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Names of secrets the stack should auto-generate.",
			},
			"custom_stacks": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Vendor-defined custom stacks, install-override-merged and parameter-rendered, in deployment order.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":             schema.StringAttribute{Computed: true, MarkdownDescription: "Raw config name, not a sanitized logical ID — it keys the phone-home payload."},
						"index":            schema.Int64Attribute{Computed: true, MarkdownDescription: "Deployment sequence index."},
						"parameters":       schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Rendered parameters passed to the stack."},
						"module":           schema.StringAttribute{Computed: true, MarkdownDescription: "Curated gcp-terraform module name. Empty for aws-cloudformation and azure-bicep stacks."},
						"outputs":          schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maps each output key the stack's template declares to the flat top-level output name the generated custom-stacks template emits for it."},
						"input_parameters": schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maps each top-level template parameter name to the install input name whose current value should be passed for it."},
					},
				},
			},

			"custom_stacks_template_url": schema.StringAttribute{Computed: true, MarkdownDescription: "URL of the generated template containing only the install's custom nested stacks. Empty when the install declares no custom stacks."},

			"secrets": schema.MapNestedAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Customer-supplied secrets, keyed by name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Secret description."},
						"required":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the secret is required."},
						"value":       schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Secret value."},
					},
				},
			},

			"gcp": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "GCP-specific configuration. Present when cloud is gcp.",
				Attributes: map[string]schema.Attribute{
					"project_id": schema.StringAttribute{Computed: true, MarkdownDescription: "GCP project the stack is provisioned into. Empty until the install has a recorded project, so the module's `project_id` variable can supply it on a first apply."},
					"region":     schema.StringAttribute{Computed: true, MarkdownDescription: "GCP region the stack is provisioned into. Empty until the install has a recorded region, so the module's `region` variable can supply it on a first apply."},

					"runner_init_script_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Runner bootstrap script URL."},
					"runner_api_token":       schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Runner API token."},
					"runner_machine_type":    schema.StringAttribute{Computed: true, MarkdownDescription: "GCE machine type for the runner instance."},

					"provision_permissions":       schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Provision service-account permissions."},
					"provision_predefined_role":   schema.StringAttribute{Computed: true, MarkdownDescription: "Provision predefined role, if any."},
					"maintenance_permissions":     schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maintenance service-account permissions."},
					"maintenance_predefined_role": schema.StringAttribute{Computed: true, MarkdownDescription: "Maintenance predefined role, if any."},
					"deprovision_permissions":     schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Deprovision service-account permissions."},
					"deprovision_predefined_role": schema.StringAttribute{Computed: true, MarkdownDescription: "Deprovision predefined role, if any."},

					"provision_policies":   schema.MapAttribute{Computed: true, ElementType: types.ListType{ElemType: types.StringType}, MarkdownDescription: "Per-policy provision custom roles (policy name → permissions)."},
					"maintenance_policies": schema.MapAttribute{Computed: true, ElementType: types.ListType{ElemType: types.StringType}, MarkdownDescription: "Per-policy maintenance custom roles (policy name → permissions)."},
					"deprovision_policies": schema.MapAttribute{Computed: true, ElementType: types.ListType{ElemType: types.StringType}, MarkdownDescription: "Per-policy deprovision custom roles (policy name → permissions)."},

					"break_glass_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Break-glass roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: roleAttrs},
					},
					"custom_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Customer-defined roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: roleAttrs},
					},
				},
			},

			"azure": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Azure-specific configuration. Present when cloud is azure.",
				Attributes: map[string]schema.Attribute{
					"location":               schema.StringAttribute{Computed: true, MarkdownDescription: "Azure location the stack is provisioned into."},
					"subscription_id":        schema.StringAttribute{Computed: true, MarkdownDescription: "Subscription the install belongs to. The module compares this against the azurerm provider's own subscription."},
					"subscription_tenant_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Tenant the subscription belongs to."},

					"runner_vm_size":      schema.StringAttribute{Computed: true, MarkdownDescription: "VM size for the runner scale set."},
					"container_image_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Runner container image URL, written as the runner's initial image config. There is no runner API token: the Azure runner authenticates as its own managed identity."},
					"container_image_tag": schema.StringAttribute{Computed: true, MarkdownDescription: "Runner container image tag."},

					"provision_actions":          schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Provision identity actions, granted via a custom role definition."},
					"provision_built_in_roles":   schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Provision identity built-in role GUIDs."},
					"maintenance_actions":        schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maintenance identity actions."},
					"maintenance_built_in_roles": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maintenance identity built-in role GUIDs."},
					"deprovision_actions":        schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Deprovision identity actions."},
					"deprovision_built_in_roles": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Deprovision identity built-in role GUIDs."},

					"break_glass_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Break-glass roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: azureRoleAttrs},
					},
					"custom_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Customer-defined roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: azureRoleAttrs},
					},
				},
			},

			"aws": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "AWS-specific configuration. Present when cloud is aws.",
				Attributes: map[string]schema.Attribute{
					"region":                     schema.StringAttribute{Computed: true, MarkdownDescription: "AWS region the stack is provisioned into."},
					"cluster_name":               schema.StringAttribute{Computed: true, MarkdownDescription: "Resolved EKS cluster-name tag value."},
					"runner_machine_type":        schema.StringAttribute{Computed: true, MarkdownDescription: "EC2 instance type for the runner host."},
					"nuon_support_iam_role_arns": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Nuon control-plane IAM role ARNs allowed to assume the operation roles."},

					"provision_permissions":              schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Provision role inline-policy IAM actions."},
					"provision_inline_policy_document":   schema.StringAttribute{Computed: true, MarkdownDescription: "Provision role inline policy document JSON."},
					"provision_managed_policy_arns":      schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs attached to the provision role."},
					"maintenance_permissions":            schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maintenance role inline-policy IAM actions."},
					"maintenance_inline_policy_document": schema.StringAttribute{Computed: true, MarkdownDescription: "Maintenance role inline policy document JSON."},
					"maintenance_managed_policy_arns":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs attached to the maintenance role."},
					"deprovision_permissions":            schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Deprovision role inline-policy IAM actions."},
					"deprovision_inline_policy_document": schema.StringAttribute{Computed: true, MarkdownDescription: "Deprovision role inline policy document JSON."},
					"deprovision_managed_policy_arns":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs attached to the deprovision role."},

					"break_glass_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Break-glass roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: awsRoleAttrs},
					},
					"custom_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Customer-defined roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: awsRoleAttrs},
					},
				},
			},
		},
	}
}

func (d *stackDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data stackDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, err := stack.FetchConfig(ctx, stack.Options{
		APIURL:    d.cfg.apiURL,
		InstallID: data.InstallID.ValueString(),
		APIToken:  d.cfg.apiToken,
		OrgID:     d.cfg.orgID,
	})
	if err != nil {
		resp.Diagnostics.AddError("fetch stack config failed", err.Error())
		return
	}

	flattenConfig(&data, cfg)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
