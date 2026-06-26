# Baseten Terraform provider

Experimental Terraform provider for Baseten custom model deployments.

## Development

```sh
nix develop
scripts/test.sh ci
```

Useful test suites:

```sh
scripts/test.sh unit
scripts/test.sh lint
scripts/test.sh vuln
scripts/test.sh build
scripts/test.sh docs
scripts/test.sh release-check
TF_ACC=1 BASETEN_API_KEY=... scripts/test.sh acceptance
```

CI runs `scripts/test.sh ci` on pushes and pull requests. Tags matching `v*`
run the release workflow, which creates draft GitHub release artifacts with
GoReleaser.

Acceptance tests provision real Baseten resources. Set
`BASETEN_ACC_ACCELERATOR` to override the default `A10G` test accelerator.

## Custom model example

See `examples/custom-model` for a complete fixture that is also exercised by
the local provider plan test.

```hcl
terraform {
  required_providers {
    baseten = {
      source = "polymath-as/baseten"
    }
  }
}

provider "baseten" {
  # Or set BASETEN_API_KEY.
  api_key = var.baseten_api_key
}

locals {
  model_path = "${path.module}/model"
  model_files = sort(fileset(local.model_path, "**"))
  model_hash = sha256(join("", [
    for file in local.model_files : filesha256("${local.model_path}/${file}")
  ]))
}

resource "baseten_custom_model" "example" {
  name        = "example-custom-model"
  source_path = local.model_path
  source_hash = local.model_hash

  raw_config  = file("${local.model_path}/config.yaml")
  config_json = jsonencode(yamldecode(file("${local.model_path}/config.yaml")))

  environment_name = "production"

  min_replica        = 0
  max_replica        = 1
  scale_down_delay   = 120
  concurrency_target = 2

  timeouts {
    create = "90m"
    update = "10m"
    delete = "10m"
  }
}
```

`min_replica = 0` enables scale-to-zero. Update `source_hash` when local model
contents change so Terraform replaces the Baseten deployment with a new archive.
Long builds can raise the `timeouts.create` value.

Existing deployments can be imported by model and deployment ID:

```sh
terraform import baseten_custom_model.example model-123:deployment-456
```

Keep the local model block configured after import; Baseten cannot reconstruct
local-only inputs such as `source_path`, `source_hash`, and `config_json`.

## Large model artifacts

Archives are streamed into Baseten's temporary S3 upload location with multipart
upload, so the provider does not keep the full tarball in memory. For models
with hundreds of GB of weights, prefer keeping weights in external storage or a
model registry and referencing them from `config.yaml`; use the Terraform upload
path for deployment code and smaller artifacts.
