package hosting

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/logging"
	"github.com/firstkb/sc-api/internal/options"
)

type HostedService interface {
	// Entry point for the service. The service will stop when Run ends.
	// Use Run to handle all initialization of the service.
	// If Stop releases resources allocated somewhere else rather than in Run,
	// the needed resources would not be created again the second time the Run is called.
	Run(ctx context.Context, config *config.Config, logger *slog.Logger) error

	// Signal the service to stop. The implementation should not call os.Exit directly.
	Stop(logger *slog.Logger)
}

type ServiceHost struct {
	Name        string
	DisplayName string
	Description string
	Version     string
	Config      *config.Config
	Logger      *slog.Logger
}

func (svcHost *ServiceHost) Run(hostedService *HostedService, args []string) {
	svcHost.RunService(hostedService, args)
}

func (svcHost *ServiceHost) Initialize(options *options.Options, defaultConfiguration []byte, envPrefixes []string) []string {
	setWorkingDirectory()

	configOptions := config.AddConfigOptions(options)
	loggerOptions := logging.AddLoggerOptions(options)

	args := options.Parse(svcHost.Version, svcHost.DisplayName, UsageCommands)

	var err error = nil

	isWinSCM := isWindowsSCManager(args)

	// get logger
	if isWinSCM {
		loggerOptions.Output = ""
		loggerOptions.Console = true
	} else {
		loggerOptions.Console = loggerOptions.Console && IsTerminal()
	}

	if svcHost.Logger, err = logging.GetLogger(loggerOptions); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}

	if !isWinSCM {
		// get configuration
		if svcHost.Config, err = config.GetConfig(configOptions, defaultConfiguration, envPrefixes...); err != nil {
			svcHost.Logger.Error(fmt.Sprintf("failed to load configuration: %v\n", err))
			os.Exit(1)
		}

		svcHost.Logger.Info(fmt.Sprintf("***** %s v%s *****", svcHost.DisplayName, svcHost.Version))

		if configOptions.Source == "" {
			svcHost.Logger.Warn("no config file specified, using the default configuration")
		}

		for _, conf := range *svcHost.Config.Sources {
			svcHost.Logger.Info(fmt.Sprintf("Loaded config: %s", conf))
		}
	}

	return args
}
