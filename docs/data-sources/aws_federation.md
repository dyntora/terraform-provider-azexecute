---
page_title: "azexecute_aws_federation Data Source - AZExecute"
subcategory: "Integrations"
description: |-
  Read the public AWS OIDC federation settings for the current AZExecute tenant.
---

# azexecute_aws_federation (Data Source)

Requires provider 0.13.5 or later and the Integrations license feature. Returns public identifiers only. It does not create an integration or change AWS settings. If federation has not been configured on the AZExecute server, the read reports an error.

```hcl
data "azexecute_aws_federation" "current" {}

output "aws_federation" {
  value = {
    issuer   = data.azexecute_aws_federation.current.provider_url
    audience = data.azexecute_aws_federation.current.audience
  }
}
```

There are no arguments. Computed attributes:

- `provider_url`: tenant-specific HTTPS OIDC issuer URL.
- `audience`: AZExecute's federation application audience. This is independent of the Terraform caller's client ID.

Use these values for the IAM OIDC provider and role trust policy in the [integration setup guide](../guides/integrations.md).
