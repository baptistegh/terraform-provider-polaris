package main

import (
	"context"
	"flag"
	"log"

	"github.com/baptistegh/terraform-provider-polaris/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version string = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/baptistegh/polaris",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
