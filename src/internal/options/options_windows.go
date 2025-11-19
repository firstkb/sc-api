//go:build windows

package options

import (
	"os"
	"path/filepath"

	"github.com/firstkb/sc-api/internal/utils"
)

// <application_path>/.env
func getGlobalEnvFilePath() (string, error) {
	appDir := utils.GetAppDir()

	return filepath.Abs(filepath.Join(appDir, defaultEnvFileName))
}

// %USERPROFILE%/<appname>/.env
func getLocalEnvFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(homeDir, utils.GetAppName(), defaultEnvFileName))
}
