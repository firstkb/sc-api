//go:build (darwin || freebsd || openbsd || netbsd || dragonfly || hurd) && !appengine && !tinygo

package logging

import (
	"errors"

	"github.com/phuslu/log"
)

func getJournalWriter() (log.Writer, error) {
	return nil, errors.New("journal is not supported on Linux platform")
}
