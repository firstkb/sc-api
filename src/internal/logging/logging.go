package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/phuslu/log"

	"github.com/firstkb/sc-api/internal/utils"
)

const (
	timeFormat = "2006-01-02T15:04:05.000Z0700"
)

func GetLogger(loggerOptions *loggerOptions) (*slog.Logger, error) {
	writers := make([]log.Writer, 0, 2)

	if loggerOptions.Output != "" {
		var writer log.Writer
		var err error = nil

		if strings.EqualFold("stdout", loggerOptions.Output) {
			loggerOptions.Console = false
			writer = &log.IOWriter{Writer: os.Stdout}
		} else if strings.EqualFold("stderr", loggerOptions.Output) {
			loggerOptions.Console = false
			writer = &log.IOWriter{Writer: os.Stderr}
		} else if strings.EqualFold("syslog", loggerOptions.Output) || strings.Contains(strings.ToLower(loggerOptions.Output), "syslog=") { // Unix only
			writer, err = getSyslogWriter(loggerOptions.Output)
		} else if strings.EqualFold("journal", loggerOptions.Output) { // Linux only
			writer, err = getJournalWriter()
		} else if strings.EqualFold("eventlog", loggerOptions.Output) { // WIndows only
			writer, err = getEventLogWriter()
		} else if !strings.EqualFold(devNull, loggerOptions.Output) && !strings.EqualFold("discard", loggerOptions.Output) {
			logPath := loggerOptions.Output
			if logPath == "." || logPath == ".." || filepath.Clean(logPath) == filepath.Dir(logPath) {
				logPath = filepath.Join(logPath, utils.GetAppName()+".log")
			}
			writer, err = getFileWriter(logPath)
		}

		if err != nil {
			return slog.Default(), err
		}

		if writer != nil {
			writers = append(writers, writer)
		}
	}

	if loggerOptions.Console {
		writers = append(writers, &log.ConsoleWriter{Formatter: consoleFormatter})
	}

	var writer log.Writer

	if len(writers) == 0 {
		if loggerOptions.Debug {
			slog.SetLogLoggerLevel(slog.LevelDebug)
		}

		return slog.Default(), nil
	}

	if len(writers) == 1 {
		writer = writers[0]
	} else {
		multiEntryWriter := make(log.MultiEntryWriter, len(writers))
		copy(multiEntryWriter, writers)
		writer = &multiEntryWriter
	}

	level := log.InfoLevel
	if loggerOptions.Debug {
		level = log.DebugLevel
	}

	return (&log.Logger{
		Level:      level,
		TimeFormat: timeFormat,
		Caller:     1,
		Writer:     writer,
	}).Slog(), nil
}

/*
FileWriter creates a symlink to the current logging file, it requires administrator privileges on Windows.

By default, only administrators can create symbolic links, because they are the only ones who have the 'SeCreateSymbolicLinkPrivilege'
privilege found under 'Computer Configuration\Windows Settings\Security Settings\Local Policies\User Rights Assignment\' granted.const

To enable using Symbolic Links the user must be given the "Create Symbolic Links" privilege or be in a group that has been given that privilege.
This setting is defined by within the 'Local Security Policies' for 'User Rights Assignment', 'Security Setting' for 'Create symbolic links' policy.
Open "Control Panel'->'Administrative Tools' and open 'Local Security Policy'. From there, open 'Local Policies'->'User Rights Assignment'.
By default, the 'Administrators' group has this privilege. For users not in the 'Administrators' group, add the user.

To refresh Group Policy settings, including security settings, run:

	gpupdate /force

Note:

	For users within the Administrators group and with UAC on, the user still must "Run as Administrator".
	It is normal UAC behavior: When a user belonging the Administrators group logs on,
	Windows creates a token representing the standard-user version of the user’s administrative identity.
	The new token is stripped of all the privileges assigned to the user except the default standard user privileges
	(Bypass traverse checking, Shut down the system, Remove computer from docking station, Increase a process working set, and
	Change the time zone). The 'Create symbolic links' is not in this list, so it is stripped out, even if it is enabled to the account.
*/
func getFileWriter(logPath string) (log.Writer, error) {
	logPath, err := filepath.Abs(logPath)
	if err != nil {
		return nil, err
	}

	var writer log.Writer = &log.FileWriter{
		Filename:     logPath,
		EnsureFolder: true,
		HostName:     true,
		MaxSize:      5 * 1024 * 1024,
		MaxBackups:   30,
		LocalTime:    true,
		TimeFormat:   "2006-01-02T15-04-05Z0700", // ISO-8601
	}

	return writer, nil
}
