//go:build windows

package config

import (
	"os"
	"path/filepath"

	"github.com/firstkb/sc-api/internal/utils"
)

// <application_path>/<appname>.conf
func getGlobalConfigFilePath() (string, error) {
	appDir := utils.GetAppDir()

	return filepath.Abs(filepath.Join(appDir, utils.GetAppName()+defaultConfigFileExt))
}

// %USERPROFILE%/<appname>/<appname>.conf
func getLocalConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(homeDir, utils.GetAppName(), utils.GetAppName()+defaultConfigFileExt))
}
