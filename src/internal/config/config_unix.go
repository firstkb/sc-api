//go:build !windows && !nacl && !plan9

package config

import (
	"os"
	"path/filepath"

	"github.com/firstkb/sc-api/internal/utils"
)

// /etc/<appname>/<appname>.conf
func getGlobalConfigFilePath() (string, error) {
	return filepath.Abs(filepath.Join("/etc/", utils.GetAppName(), utils.GetAppName()+defaultConfigFileExt))
}

// $HOME/.config/<appname>/<appname>.conf
func getLocalConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(homeDir, ".config", utils.GetAppName(), utils.GetAppName()+defaultConfigFileExt))
}
