# Baseten Terraform provider

Experimental Terraform provider for Baseten custom model deployments.

## Development

```sh
nix develop
go test ./...
golangci-lint run
```

## Custom model example

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

  min_replica        = 0
  max_replica        = 1
  scale_down_delay   = 120
  concurrency_target = 2
}
```

`min_replica = 0` enables scale-to-zero. Update `source_hash` when local model
contents change so Terraform replaces the Baseten deployment with a new archive.
