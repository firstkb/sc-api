package options

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/firstkb/sc-api/internal/utils"
)

type commandOptions struct {
	AppName string

	flagSet  *flag.FlagSet
	envNames map[string]string
}

func NewCommandOptions() *commandOptions {
	options := &commandOptions{
		AppName: utils.GetAppName(),
	}

	options.flagSet = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	return options
}

func NewCommandOptionsWithEnv(envNames map[string]string) *commandOptions {
	options := NewCommandOptions()

	if len(envNames) > 0 {
		options.envNames = make(map[string]string)

		for key, value := range envNames {
			options.envNames[strings.ToLower(key)] = value
		}
	}

	return options
}

func (options *commandOptions) Parse(args []string, serviceVersion string, serviceDescription string, usageOptions string) {
	if serviceDescription == "" {
		serviceDescription = strings.ToUpper(options.AppName)
	}

	showHelp := options.flagSet.Bool("h", false, "")
	showHelpLong := options.flagSet.Bool("help", false, "")

	options.flagSet.Usage = func() {}

	var err = options.flagSet.Parse(args[1:])
	if err != nil {
		os.Exit(2)
	}

	if *showHelp || *showHelpLong {
		options.printUsage(os.Stdout, serviceVersion, serviceDescription, args[0], usageOptions)
		os.Exit(0)
	}
}

func (options *commandOptions) Bool(name string, value bool, usage string) *bool {
	return options.flagSet.Bool(name, envValueOrDefaultBool(options.envNames, name, value), usage)
}

func (options *commandOptions) String(name string, value string, usage string) *string {
	return options.flagSet.String(name, envValueOrDefaultString(options.envNames, name, value), usage)
}

func (options *commandOptions) printUsage(w io.Writer, serviceVersion string, serviceDescription string, cmd string, usageOptions string) {
	if !strings.EqualFold("create", cmd) && !strings.EqualFold("delete", cmd) {
		return
	}

	appName := utils.GetAppName()

	if serviceDescription == "" {
		serviceDescription = strings.ToUpper(appName)
	}

	version := serviceDescription + " v" + serviceVersion

	opts := ""
	if len(usageOptions) != 0 {
		opts = " [options]"
	}

	fmt.Fprintf(w,
		`%s

Usage: 
  %s %s%s

`, version, appName, cmd, opts)

	if len(usageOptions) == 0 {
		return
	}

	fmt.Fprintf(w,
		`Options:%s
`, usageOptions)
}
