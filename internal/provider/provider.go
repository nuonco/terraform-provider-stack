package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultAPIURL = "https://runner.nuon.co"

// apiURLEnvVar mirrors the credential attributes, which fall back to the
// environment. Without it api_url is the one setting that silently defaults to
// production, so pointing at a local control plane means editing config that was
// generated for a customer.
const apiURLEnvVar = "NUON_API_URL"

// stackProvider is the Nuon Terraform provider.
type stackProvider struct {
	version string
}

// providerConfig is the resolved provider-level configuration handed to each
// resource via Configure.
type providerConfig struct {
	apiURL   string
	apiToken string
	orgID    string
}

var _ provider.Provider = (*stackProvider)(nil)

// New returns a provider factory for the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &stackProvider{version: version}
	}
}

func (p *stackProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "stack"
	resp.Version = p.version
}

func (p *stackProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read Nuon install-stack configuration and report run status from Terraform.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base URL of the Nuon runner API, up to but excluding `/v1`. Falls back to `" + apiURLEnvVar + "`, then `" + defaultAPIURL + "`.",
			},
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Nuon API token, issued by the vendor alongside the install. Falls back to `NUON_API_TOKEN`. If neither is set, the provider looks for an ambient OIDC token (GitHub Actions with `permissions: id-token: write`, `NUON_OIDC_TOKEN`, or `NUON_OIDC_TOKEN_FILE`) and exchanges it for a short-lived token.",
			},
			"org_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Nuon organization ID. Required only when authenticating via OIDC, where the exchange must name the org whose trust policies apply. Falls back to `NUON_ORG_ID`.",
			},
		},
	}
}

type providerModel struct {
	APIURL   types.String `tfsdk:"api_url"`
	APIToken types.String `tfsdk:"api_token"`
	OrgID    types.String `tfsdk:"org_id"`
}

func (p *stackProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiURL := defaultAPIURL
	if env := strings.TrimSpace(os.Getenv(apiURLEnvVar)); env != "" {
		apiURL = env
	}
	if !data.APIURL.IsNull() && data.APIURL.ValueString() != "" {
		apiURL = data.APIURL.ValueString()
	}

	// Empty values are passed through rather than defaulted here: the SDK owns the
	// fallback chain (explicit, then environment, then OIDC exchange), and resolving
	// it in two places would let the two disagree.
	cfg := &providerConfig{
		apiURL:   apiURL,
		apiToken: data.APIToken.ValueString(),
		orgID:    data.OrgID.ValueString(),
	}
	resp.ResourceData = cfg
	resp.DataSourceData = cfg
}

func (p *stackProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewPhoneHomeResource,
	}
}

func (p *stackProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewStackDataSource,
	}
}
