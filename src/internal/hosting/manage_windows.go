// build only on windows
//go:build windows

// https://cs.opensource.google/go/x/sys/+/master:windows/svc/example/install.go

package hosting

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/firstkb/sc-api/internal/options"
	"github.com/firstkb/sc-api/internal/utils"
)

const (
	cmdCreate = "create"
	cmdDelete = "delete"

	flagStart    = "start"
	flagAccount  = "account"
	flagPassword = "password"
	flagForce    = "force"
)

const UsageCommands = `
  %[1]s [global options] <command> [options]

Commands:
  ` + cmdCreate + `   Register the server as a Windows service (add to the registry)
  ` + cmdDelete + `   Unregister the service (delete from the registry)

For more information, run any command with the '--help' flag
`

const usageOptionsCreate = `
  --` + flagStart + `           Start the service after the registration is complete
  --` + flagAccount + ` <acc>   An account to run the service (default: LocalSystem)
  --` + flagPassword + ` <psw>  A password to use when an account other than the LocalSystem
`
const usageOptionsDelete = `
    --` + flagForce + `  Forces a running service to stop
`

func createService(svcHost *ServiceHost, args []string) error {
	commandOptions := options.NewCommandOptions()

	startAfterCreate := commandOptions.Bool(flagStart, false, "")
	account := commandOptions.String(flagAccount, "LocalSystem", "")
	password := commandOptions.String(flagPassword, "", "")

	commandOptions.Parse(args, svcHost.Version, svcHost.DisplayName, usageOptionsCreate)

	exePath, err := utils.GetExecutable()
	if err != nil {
		return err
	}

	manager, err := mgr.Connect()
	if err != nil {
		if strings.Contains(err.Error(), "Access is denied") {
			return errors.New("you must be an administrator to create the service")
		}
		return fmt.Errorf("unable to connect to Windows Service Control Manager: %w", err)
	}
	defer manager.Disconnect()

	svcHost.Logger.Debug(cmdCreate, "exePath", exePath, "account", account, "start", startAfterCreate)

	service, err := manager.OpenService(svcHost.Name)
	if err == nil {
		service.Close()
		return errors.New("service is already created")
	}

	cfg := mgr.Config{
		ServiceType:      windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:        mgr.StartAutomatic,
		ErrorControl:     mgr.ErrorNormal,
		DisplayName:      svcHost.DisplayName,
		Description:      svcHost.Description,
		ServiceStartName: *account,
		//Arguments:        []string{"--config=" + configFile, "--log" + logFile},
	}

	if len(*password) > 0 {
		cfg.Password = *password
	}

	service, err = manager.CreateService(svcHost.Name, exePath, cfg)
	if err != nil {
		return err
	}
	defer service.Close()

	// restart after:
	// - 1 second for the first time
	// - 5 seconds for the second
	// - 1 minute, otherwise
	ra := []mgr.RecoveryAction{
		{
			Type:  mgr.ServiceRestart,
			Delay: 1000 * time.Millisecond,
		},
		{
			Type:  mgr.ServiceRestart,
			Delay: 5000 * time.Millisecond,
		},
		{
			Type:  mgr.ServiceRestart,
			Delay: 60000 * time.Millisecond,
		},
	}

	err = service.SetRecoveryActions(ra, 86400) // 1 day reset period
	if err != nil {
		return fmt.Errorf("unable to set recovery action: %w", err)
	}

	svcHost.Logger.Info(fmt.Sprintf("service %s has been successfully created", svcHost.Name))

	if *startAfterCreate {
		err = service.Start()
		if err != nil {
			svcHost.Logger.Warn(fmt.Sprintf("failed to start the service %s: %v", svcHost.Name, err))
		}
	}

	return nil
}

func deleteService(svcHost *ServiceHost, args []string) error {
	commandOptions := options.NewCommandOptions()

	forceToDelete := commandOptions.Bool(flagForce, false, "")

	commandOptions.Parse(args, svcHost.Version, svcHost.DisplayName, usageOptionsDelete)

	manager, err := mgr.Connect()
	if err != nil {
		if strings.Contains(err.Error(), "Access is denied") {
			return errors.New("you must be an administrator to delete the service")
		}
		return fmt.Errorf("unable to connect to Windows Service Control Manager: %w", err)
	}
	defer manager.Disconnect()

	svcHost.Logger.Debug(cmdDelete, flagForce, forceToDelete)

	service, err := manager.OpenService(svcHost.Name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return errors.New("service does not exist as an installed service")
		}
		return fmt.Errorf("OpenService returned an error: %w", err)
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return err
	}

	if status.State == svc.Running {
		if *forceToDelete {
			_, err = service.Control(svc.Stop)
			if err != nil {
				return fmt.Errorf("unable to stop the service: %w", err)
			}
		} else {
			return errors.New("service is in Running state")
		}
	}

	err = service.Delete()
	if err != nil {
		return err
	}

	svcHost.Logger.Info(fmt.Sprintf("service %s has been successfully deleted", svcHost.Name))

	return nil
}
