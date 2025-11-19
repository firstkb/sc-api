//go:build windows

package logging

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/phuslu/log"

	"github.com/firstkb/sc-api/internal/utils"
)

const (
	devNull = "NUL"

	logDescription = " <path>|eventlog\vUse a custom log location or redirect to Event log"
)

// %USERPROFILE%/<appname>/log
func getDefaultLogFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.Contains(strings.ToLower(homeDir), "system32") {
		homeDir = "./"
	} else {
		homeDir = filepath.Join(homeDir, utils.GetAppName())
	}

	return filepath.Join(homeDir, "log")
}

func getEventLogWriter() (log.Writer, error) {

	// TODO: eventlog

	/*
		&log.EventlogWriter{
			Source: ".NET Runtime",
			ID:     1000,
		}
	*/

	return nil, errors.New("eventlog is not implemented yet")
}

func getJournalWriter() (log.Writer, error) {
	return nil, errors.New("journal is not supported on Windows platform")
}

func getSyslogWriter(_ string) (log.Writer, error) {
	return nil, errors.New("syslog is not supported on Windows platform")
}
