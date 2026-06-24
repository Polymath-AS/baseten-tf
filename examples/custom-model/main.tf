terraform {
  required_providers {
    baseten = {
      source = "polymath-as/baseten"
    }
  }
}

provider "baseten" {}

locals {
  model_path  = "${path.module}/model"
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
