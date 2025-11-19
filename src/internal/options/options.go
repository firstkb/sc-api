package options

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/firstkb/sc-api/internal/utils"
)

const (
	flagEnv = "env"
)

const (
	defaultEnvFileName = ".env"
)

var (
	envPrefix = strings.ToUpper(utils.GetAppName()) + "_"
)

/*
Precedence of options (highest to lowest):
  - Command line
  - An env file thats name is declared on the command line (will override existing environment variables)
  - Environment variables
  - Local .env file (if exists)
    linux  : $HOME/.config/<appname>/.env
    windows: %USERPROFILE%/<appname>/.env
  - Global .env file (if exists)
    linux  : /etc/<appname>/.env
    windows: <application_path>/.env
  - Hardcoded default values
*/
type Options struct {
	flagSet    *flag.FlagSet
	envNames   map[string]string
	usage      map[string]string
	usageOrder []string
}

func NewOptions(envNamePrefix string) (*Options, error) {
	if envNamePrefix != "" {
		envPrefix = strings.ToUpper(envNamePrefix) + "_"
	}

	options := &Options{}

	options.flagSet = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	options.envNames = make(map[string]string)
	options.usage = make(map[string]string)

	flagSet := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	cmdEnvFilePath := flagSet.String(flagEnv, "", " ") // this option overrides existing environment variables

	flagSet.Usage = func() {}
	flagSet.SetOutput(io.Discard)
	flagSet.Parse(os.Args[1:])

	if *cmdEnvFilePath != "" {
		filePath, err := filepath.Abs(*cmdEnvFilePath)
		if err != nil {
			return nil, err
		}
		err = godotenv.Overload(filePath)
		if err != nil {
			return nil, err
		}
	}

	filePath, err := getLocalEnvFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load local .env: %v\n", err)
	} else {
		err = godotenv.Load(filePath)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "failed to load local .env: %v\n", err)
		}
	}

	filePath, err = getGlobalEnvFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load global .env: %v\n", err)
	} else {
		err = godotenv.Load(filePath)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "failed to load global .env: %v\n", err)
		}
	}

	return options, nil
}

func (options *Options) Parse(serviceVersion string, serviceDescription string, usageCommands string) []string {
	if serviceDescription == "" {
		serviceDescription = strings.ToUpper(utils.GetAppName())
	}

	if options.usage[flagEnv] == "" {
		_ = options.String(flagEnv, "", " <path>\vCustom env file path", false)
	}

	showHelp := options.flagSet.Bool("h", false, "")
	showHelpLong := options.flagSet.Bool("help", false, "")
	showVersion := options.flagSet.Bool("v", false, "")
	showVersionLong := options.flagSet.Bool("version", false, "")

	options.flagSet.SetOutput(io.Discard)
	options.flagSet.Usage = func() {}

	var err = options.flagSet.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(2)
	}

	if *showHelp || *showHelpLong {
		printVersion(os.Stdout, serviceDescription, serviceVersion)
		fmt.Fprintln(os.Stdout)
		options.printUsage(os.Stdout, usageCommands)
		os.Exit(0)
	}

	if *showVersion || *showVersionLong {
		printVersion(os.Stdout, "", serviceVersion)
		os.Exit(0)
	}

	return options.flagSet.Args()
}

func (options *Options) Int(name string, value int, usage string, isEnvVar bool) *int {
	if isEnvVar {
		options.addEnvName(name)
	}

	if usage != "" {
		options.usage[name] = usage
		options.usageOrder = append(options.usageOrder, name)
	}

	return options.flagSet.Int(name, envValueOrDefaultInt(options.envNames, name, value), "")
}

func (options *Options) IntVar(p *int, name string, value int, usage string, isEnvVar bool) {
	if isEnvVar {
		options.addEnvName(name)
	}

	if usage != "" {
		options.usage[name] = usage
		options.usageOrder = append(options.usageOrder, name)
	}

	options.flagSet.IntVar(p, name, envValueOrDefaultInt(options.envNames, name, value), "")
}

func (options *Options) Bool(name string, value bool, usage string, isEnvVar bool) *bool {
	if isEnvVar {
		options.addEnvName(name)
	}

	if usage != "" {
		options.usage[name] = usage
		options.usageOrder = append(options.usageOrder, name)
	}

	return options.flagSet.Bool(name, envValueOrDefaultBool(options.envNames, name, value), "")
}

func (options *Options) BoolVar(p *bool, name string, value bool, usage string, isEnvVar bool) {
	if isEnvVar {
		options.addEnvName(name)
	}

	if usage != "" {
		options.usage[name] = usage
		options.usageOrder = append(options.usageOrder, name)
	}

	options.flagSet.BoolVar(p, name, envValueOrDefaultBool(options.envNames, name, value), "")
}

func (options *Options) String(name string, value string, usage string, isEnvVar bool) *string {
	if isEnvVar {
		options.addEnvName(name)
	}

	if usage != "" {
		options.usage[name] = usage
		options.usageOrder = append(options.usageOrder, name)
	}

	return options.flagSet.String(name, envValueOrDefaultString(options.envNames, name, value), "")
}

func (options *Options) StringVar(p *string, name string, value string, usage string, isEnvVar bool) {
	if isEnvVar {
		options.addEnvName(name)
	}

	if usage != "" {
		options.usage[name] = usage
		options.usageOrder = append(options.usageOrder, name)
	}

	options.flagSet.StringVar(p, name, envValueOrDefaultString(options.envNames, name, value), "")
}

func (options *Options) addEnvName(name string) {
	options.envNames[strings.ToLower(name)] = getEnvName(envPrefix + strings.ToUpper(name))
}

func printVersion(w io.Writer, appName string, version string) {
	if appName != "" {
		version = appName + " v" + version
	}

	fmt.Fprint(w, version)
}

func (options *Options) printUsage(w io.Writer, usageCommands string) {
	appName := utils.GetAppName()

	usage := fmt.Sprintf(`
Usage:
  %[1]s`, appName)

	if usageCommands == "" {
		usage += " [options]\n"
	} else {
		usage += " [global options]"
	}

	fmt.Fprint(w, usage)

	if usageCommands != "" {
		fmt.Fprintf(w, usageCommands, appName)
	}

	fmt.Fprintln(w)

	if usageCommands == "" {
		fmt.Fprintln(w, "Options:")
	} else {
		fmt.Fprintln(w, "Global options:")
	}

	options.printUsageOptions(w)

	fmt.Fprintf(w, "\n")

	if len(options.envNames) > 0 {
		fmt.Fprintln(w, "The following environment variables are supported:")

		for _, k := range options.usageOrder {
			value := options.envNames[k]
			if value != "" {
				fmt.Fprintf(w, "  %s\n", value)
			}
		}
		fmt.Fprintln(w)
	}
}

func (options *Options) printUsageOptions(w io.Writer) {
	if len(options.usage) == 0 {
		return
	}

	usage := make(map[string]string)
	for k, v := range options.usage {
		usage[k] = v
	}

	flag := make([]string, 0, len(options.usage))
	desc := make([]string, 0, len(options.usage))

	const minFlagLength = 16
	const optionPrefix = "  --"

	maxFlagLen := 0

	for _, k := range options.usageOrder {
		v := options.usage[k]

		if len(v) == 0 {
			continue
		}

		f := optionPrefix + k

		sa := strings.Split(v, "\v")
		if len(sa) == 1 {
			desc = append(desc, sa[0])
		} else {
			f += sa[0]
			desc = append(desc, sa[1])
		}

		flag = append(flag, f)

		if len(f) > maxFlagLen && len(f) < 40 {
			maxFlagLen = len(f)
		}
	}

	if maxFlagLen < minFlagLength {
		maxFlagLen = minFlagLength
	}

	for i := 0; i < len(flag); i++ {
		fmt.Fprintf(w, "%-"+strconv.Itoa(maxFlagLen)+"s  %s\n", flag[i], desc[i])
	}
}

func getEnvName(name string) string {
	return strings.ReplaceAll(strings.ToUpper(name), "-", "_")
}

func envValueOrDefaultInt(envNames map[string]string, name string, defaultValue int) int {
	name = envNames[strings.ToLower(name)]
	if name != "" {
		value, err := strconv.Atoi(strings.ToLower(os.Getenv(name)))
		if err == nil {
			return value
		}
	}

	return defaultValue
}

func envValueOrDefaultBool(envNames map[string]string, name string, defaultValue bool) bool {
	name = envNames[strings.ToLower(name)]
	if name != "" {
		value, err := strconv.ParseBool(strings.ToLower(os.Getenv(name)))
		if err == nil {
			return value
		}
	}

	return defaultValue
}

func envValueOrDefaultString(envNames map[string]string, name string, defaultValue string) string {
	name = envNames[strings.ToLower(name)]
	if name != "" {
		value := os.Getenv(name)
		if value != "" {
			return value
		}
	}

	return defaultValue
}
