//go:build !windows && !nacl && !plan9

package options

import (
	"os"
	"path/filepath"

	"github.com/firstkb/sc-api/internal/utils"
)

// /etc/<appname>/.env
func getGlobalEnvFilePath() (string, error) {
	return filepath.Abs(filepath.Join("/etc/", utils.GetAppName(), defaultEnvFileName))
}

// $HOME/.config/<appname>/.env
func getLocalEnvFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(homeDir, ".config", utils.GetAppName(), defaultEnvFileName))
}
