package main

import (
	"context"
	"flag"
	"log"

	"github.com/dyntora/terraform-provider-azexecute/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/dyntora/azexecute",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
