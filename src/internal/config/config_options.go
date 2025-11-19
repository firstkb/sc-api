package config

import (
	"github.com/firstkb/sc-api/internal/options"
)

const (
	flagConfig        = "config"
	flagConfigShort   = "c"
	configDescription = " <source>\vUse a custom config file or a Parameter Store"
)

type configOptions struct {
	Source string
}

/*
[file://]<filepath>.conf|.toml|.json|.yaml|
ssm://<path>
ssm+conf://<name>
ssm+toml://<name>
ssm+json://<name>
ssm+yaml://<name>
*/
func AddConfigOptions(appOptions *options.Options) *configOptions {
	configOptions := &configOptions{}

	// NOTE: do not change the order
	appOptions.StringVar(&configOptions.Source, flagConfigShort, "", "", false)
	appOptions.StringVar(&configOptions.Source, flagConfig, "", configDescription, true)

	return configOptions
}
