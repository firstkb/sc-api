//go:build !windows && !nacl && !plan9

package hosting

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
)

const UsageCommands = ``

func (svcHost *ServiceHost) RunService(service *HostedService, args []string) {
	//  Invalid command line
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "invalid command: %s\n", args[0])
		os.Exit(2)
	}

	// sets a panic handler before starting the service
	defer func() {
		if r := recover(); r != nil {
			svcHost.Logger.Error(fmt.Sprintf("fatal error: %v\n%v", r, string(debug.Stack())))

			os.Exit(1)
		}
	}()

	// sets signal handlers for graceful shutdown the application
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx,
		syscall.SIGHUP,  // (“hang-up”) signal is used to report that the user's terminal is disconnected
		syscall.SIGINT,  // (“program interrupt”) signal is sent when the user types the INTR character (normally C-c)
		syscall.SIGQUIT, // signal is similar to SIGINT, except that it’s controlled by a different key—the QUIT character, usually C-\
		syscall.SIGTERM) // signal is a generic signal used to cause program termination. The shell command 'kill' generates SIGTERM by default
	defer cancel()

	err := (*service).Run(ctx, svcHost.Config, svcHost.Logger)
	if err == nil {
		//....
	} else {
		svcHost.Logger.Error(fmt.Sprintf("service exited with error: %v", err))
		os.Exit(1)
	}
}

func isWindowsSCManager(_ []string) bool {
	return false
}

func setWorkingDirectory() {
}
