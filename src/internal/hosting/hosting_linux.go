//go:build linux && !appengine && !tinygo

package hosting

import (
	"os"

	"golang.org/x/sys/unix"
)

func IsTerminal() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	return err == nil
}
