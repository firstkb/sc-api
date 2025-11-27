package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/firstkb/sc-api/cmd/scapi/internal"
	"github.com/firstkb/sc-api/internal/hosting"
	"github.com/firstkb/sc-api/internal/options"
)

// These variables are replaced by ldflags at build time:
// go build -ldflags="-X 'main.Version=1.0.0' -X 'main.Build=$(date +%s)'"
var (
	Version = "0.0.0" // https://go.dev/doc/modules/version-numbers
	Build   = "dev"
)

var serviceHost = hosting.ServiceHost{
	Name:        "SCAPI",
	DisplayName: "SafeConstructors API",
	Description: "SafeConstructors API Server",
	Version:     Version,
	Logger:      slog.Default(),
}

// Prefixes to filter the env vars. The envPrefixes[0] will be stripped from the key
// Examples:
//
//	prefixes: "SCAPI_", "AWS_"
//	env name: "SCAPI_TLS_CERT", "AWS_S3_ACCESS_KEY"
//	key: 	  tls.cert, aws.s3.access.key
var envPrefixes = []string{serviceHost.Name + "_", "AWS_"}

var defaultConfiguration []byte // The default configuration settings in TOML format

var apiServer hosting.HostedService = &internal.HostedService{}

func init() {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set time zone: %v\n", err)
	} else {
		time.Local = loc
	}
}

func main() {

	if Build != "" {
		serviceHost.Version = Version + "." + Build
		os.Setenv("MIGRATION_VERSION", serviceHost.Version)
	}

	// get options
	options, err := options.NewOptions(serviceHost.Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	host := options.String(internal.FlagHost, "", " [address][:port]\vIP address and/or port to listen on", true)

	// parse options, create logger and load configuration
	args := serviceHost.Initialize(options, defaultConfiguration, envPrefixes)

	if len(*host) > 0 && serviceHost.Config != nil {
		serviceHost.Config.Set(internal.FlagHost, *host)
	}

	// run service
	serviceHost.Run(&apiServer, args)
}
