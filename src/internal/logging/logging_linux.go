//go:build linux && !appengine && !tinygo

package logging

import (
	"github.com/phuslu/log"
)

func getJournalWriter() (log.Writer, error) {
	var writer log.Writer = &log.JournalWriter{
		JournalSocket: "/run/systemd/journal/socket",
	}

	return writer, nil
}
