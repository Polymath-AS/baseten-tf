package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/polymath-as/baseten-tf/internal/provider"
)

var version = "dev"

func main() {
	err := providerserver.Serve(
		context.Background(),
		provider.New(version),
		providerserver.ServeOpts{
			Address: "registry.terraform.io/polymath-as/baseten",
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}
