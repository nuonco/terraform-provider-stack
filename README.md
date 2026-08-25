# terraform-provider-stack

The `stack` Terraform provider. It lets an install-stacks Terraform module read
its Nuon-rendered configuration from the control plane instead of receiving it
as generated tfvars.

The provider exposes two surfaces:

- **`stack_config` data source** — read-only fetch of a stack's rendered config
  (runner details, permissions, roles, install inputs, secrets) keyed by
  `install_id`. Intended for use _inside_ an install-stacks module (e.g.
  `nuonco/install-stacks//gcp`) so it reads config from the API rather than
  receiving it as generated tfvars. Provisions nothing.
- **`stack_phone_home` resource** — reports the result of a run back to the
  control plane, so the module reports run status through the provider instead
  of building the phone-home HTTP request itself. The resource lifecycle drives
  the phone-home `request_type` (Create/Update/Delete); the reported outputs are
  passed as an opaque `jsonencode({...})` payload.

The data source calls the stack SDK's read-only `FetchConfig`, which hits the
authenticated, side-effect-free `GET /v1/stacks/{install_id}/config` endpoint.
The resource calls the SDK's `PhoneHome`, which POSTs to the URL that response
returns as `phone_home_url`.

## Authentication

Both surfaces authenticate with a Nuon API token, resolved the same way the
`nuon` CLI resolves it:

1. the provider's `api_token` argument
2. `NUON_API_TOKEN`
3. an ambient OIDC token, exchanged at `/v1/oidc/token` for a short-lived token

The third path is the one to prefer in CI: GitHub Actions mints an ID token per
run (`permissions: id-token: write`), so nothing long-lived is stored. It needs
`org_id` (or `NUON_ORG_ID`), because the exchange has to name the org whose
trust policies apply.

`install_id` is an identifier, not a credential. This is the substantive change
from earlier versions, where the per-stack-version `phone_home_id` in the URL
path *was* the secret and the endpoints were public.

## Layout

```
main.go                          provider entry point (providerserver.Serve)
internal/provider/
  provider.go                    provider schema + api_url/api_token/org_id; registers the data source + resource
  stack_data_source.go           stack_config data source: schema + read
  stack_data_source_model.go     data source model + config flattener
  phone_home_resource.go         stack_phone_home resource: schema + lifecycle
  phone_home_resource_model.go   resource model
  *_test.go                      schema validation + flatten unit tests
internal/stack/                  vendored stack SDK: FetchConfig + PhoneHome (zero external deps)
examples/
  data-source-gcp/main.tf        stack_config + stack_phone_home example (GCP)
docs/
  data-source.html               architecture/walkthrough for the data source + resource
```

## Provider configuration

```hcl
provider "stack" {
  api_url = "https://runner.nuon.co" # optional; base URL up to but excluding /v1
}
```

The config endpoint lives on Nuon's runner API surface. In production `api_url`
is `https://runner.nuon.co`; for local development point it at the local runner API
(`http://localhost:8083`).

## Development

Build and install for local testing:

```bash
go build -o "$(go env GOPATH)/bin/terraform-provider-stack" .
```

Point Terraform at the local build with a dev override (`~/.terraformrc` or a
file referenced by `TF_CLI_CONFIG_FILE`):

```hcl
provider_installation {
  dev_overrides { "nuonco/stack" = "/Users/<your-home-directory>/go/bin" }
  direct {}
}
```

With a dev override set, skip `terraform init` — run `terraform plan`/`apply`
directly.

Run the tests:

```bash
go test ./...
```

See `docs/data-source.html` for the architecture diagrams, schema tables, and
step-by-step walkthroughs of both the data source and the resource.
