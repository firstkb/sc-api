package logging

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/phuslu/log"

	"github.com/firstkb/sc-api/internal/options"
	"github.com/firstkb/sc-api/internal/utils"
)

const (
	flagDebug    = "debug"
	flagLog      = "log"
	flagLogShort = "l"
)

type loggerOptions struct {
	Output  string
	Debug   bool
	Console bool
}

func AddLoggerOptions(appOptions *options.Options) *loggerOptions {
	loggerOptions := &loggerOptions{
		Console: log.IsTerminal(os.Stdout.Fd()),
		Debug:   strings.Contains(strings.ToLower(strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))), "_debug_"),
	}

	appOptions.BoolVar(&loggerOptions.Debug, flagDebug, false, "Turn on debug logging", true)
	// NOTE: do not change the order
	appOptions.StringVar(&loggerOptions.Output, flagLogShort, filepath.Join(getDefaultLogFilePath(), utils.GetAppName()+".log"), "", false)
	appOptions.StringVar(&loggerOptions.Output, flagLog, filepath.Join(getDefaultLogFilePath(), utils.GetAppName()+".log"), logDescription, true)

	return loggerOptions
}
