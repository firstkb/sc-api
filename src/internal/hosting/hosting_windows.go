//go:build windows

// https://cs.opensource.google/go/x/sys/+/master:windows/svc/example/service.go

package hosting

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/svc"

	"github.com/firstkb/sc-api/internal/utils"
)

type handler struct {
	host    *ServiceHost
	service *HostedService
}

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
)

func IsTerminal() bool {
	var st uint32
	r, _, e := syscall.SyscallN(procGetConsoleMode.Addr(), os.Stdout.Fd(), uintptr(unsafe.Pointer(&st)), 0)
	if r != 0 && e == 0 {
		isService, err := svc.IsWindowsService()
		if err == nil {
			return !isService
		}
	}

	return false
}

func isWindowsSCManager(args []string) bool {
	if len(args) == 0 {
		return false
	}

	cmd := strings.ToLower(args[0])

	return cmd == cmdCreate || cmd == cmdDelete
}

func setWorkingDirectory() {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return
	}

	wd, err := os.Getwd()
	if err != nil || strings.Contains(strings.ToLower(wd), "system32") {
		wd = utils.GetAppDir()
		if len(wd) > 0 {
			_ = os.Chdir(wd)
		}
	}
}

func (svcHost *ServiceHost) RunService(service *HostedService, args []string) {
	defer func() {
		if r := recover(); r != nil {
			svcHost.Logger.Error(fmt.Sprintf("fatal error: %v", r), "stack", utils.StackWithoutPanic(debug.Stack()))
			os.Exit(1)
		}
	}()

	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		svcHost.Logger.Error(fmt.Sprintf("failed to determine if it is running as a windows service: %v", err))
		os.Exit(1)
	}

	if len(args) == 0 {
		if isWindowsService {
			err = svcHost.runAsWindowsService(service)
		} else {
			err = svcHost.runAsConsole(service)
		}

		if err == nil {
			os.Exit(0)
		}

		svcHost.Logger.Error(fmt.Sprintf("the service exited with error: %v", err))
		os.Exit(1)
	}

	if isWindowsService {
		svcHost.Logger.Error("service control commands can only be used in the console mode")
		os.Exit(1)
	}

	cmd := strings.ToLower(args[0])

	switch cmd {
	case cmdCreate:
		err = createService(svcHost, args)
	case cmdDelete:
		err = deleteService(svcHost, args)
	default:
		svcHost.Logger.Error(fmt.Sprintf("invalid command: %s", cmd))
		os.Exit(2)
	}

	if err != nil {
		svcHost.Logger.Error(fmt.Sprintf("Failed to %s %s: %v", cmd, svcHost.Name, err))
		os.Exit(1)
	}
}

func (svcHost *ServiceHost) runAsWindowsService(service *HostedService) error {
	svcHost.Logger.Info(fmt.Sprintf("starting %s service", svcHost.Name))

	err := svc.Run(svcHost.Name, &handler{
		host:    svcHost,
		service: service,
	})
	if err == nil {
		svcHost.Logger.Info(fmt.Sprintf("%s service stopped", svcHost.Name))
	}

	return err
}

func (svcHost *ServiceHost) runAsConsole(service *HostedService) error {
	svcHost.Logger.Info("initializing...")

	ctxCancel, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctrlBreak := make(chan os.Signal, 1) // 1 is to fix 'the channel used with signal.Notify should be buffered'
	if IsTerminal() {
		signal.Notify(ctrlBreak, os.Interrupt)
	}

	serviceStopped := make(chan error)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				svcHost.Logger.Error(fmt.Sprintf("fatal error: %v", r), "stack", utils.StackWithoutPanic(debug.Stack()))
				os.Exit(1)
			}
		}()

		err := (*service).Run(ctxCancel, svcHost.Config, svcHost.Logger)

		(*service).Stop(svcHost.Logger)

		serviceStopped <- err
	}()

	select {
	case <-ctxCancel.Done():
	case <-ctrlBreak:
	case err := <-serviceStopped:
		return err
	}

	return nil
}

// Windows Service handler
func (sh *handler) Execute(args []string, cr <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	defer func() {
		if r := recover(); r != nil {
			sh.host.Logger.Error(fmt.Sprintf("fatal error in Service handler: %v", r), "stack", utils.StackWithoutPanic(debug.Stack()))
		}
	}()

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown // | svc.AcceptPauseAndContinue

	changes <- svc.Status{State: svc.StartPending}

	ctxCancel, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				sh.host.Logger.Error(fmt.Sprintf("fatal error: %v", r), "stack", utils.StackWithoutPanic(debug.Stack()))
			}
		}()

		err := (*sh.service).Run(ctxCancel, sh.host.Config, sh.host.Logger)
		if err != nil {
			sh.host.Logger.Error(fmt.Sprintf("the service exited with error: %v", err))
		}

		sh.host.Logger.Info("Service is stopping...")
		(*sh.service).Stop(sh.host.Logger)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case <-ctxCancel.Done():
			break loop
		case c := <-cr:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				cancel()
				break loop
			/*
				case svc.Pause:
					changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				case svc.Continue:
					changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			*/
			default:
				sh.host.Logger.Error(fmt.Sprintf("unexpected service control request #%d", c))
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}

	return false, 0
}
