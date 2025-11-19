package utils

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func GetAppName() string {
	appName := strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))
	if strings.Contains(strings.ToLower(appName), "_debug_") {
		dir := filepath.Base(filepath.Dir(os.Args[0]))
		if dir != "" && dir != "." {
			appName = dir
		}
	}

	return appName
}

func GetAppDir() string {
	path, err := os.Executable()

	// test for 'go run ...'
	if strings.Contains(path, "\\exe\\") && strings.Contains(path, "\\go-build") {
		path, err = filepath.Abs("") // current path
		if err != nil {
			return ""
		}
		return path
	}

	return filepath.Dir(path)
}

func GetExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}

	if strings.Contains(strings.ToLower(path), "_debug_") {
		return "", errors.New("debug mode is not supported")
	}

	// test for 'go run ...'
	if strings.Contains(path, "\\exe\\") && strings.Contains(path, "\\go-build") {
		return "", errors.New("compile and run is not supported")
	}

	return path, nil
}

func StackWithoutPanic(stack []byte) string {
	if len(stack) == 0 {
		return ""
	}

	i := bytes.Index(stack, []byte("runtime/panic.go"))
	if i != -1 {
		stack = stack[i:]
		i = bytes.Index(stack, []byte{0xA})
		if i != -1 {
			return string(stack[i+1 : len(stack)-1])
		}
	}

	return string(stack)
}
