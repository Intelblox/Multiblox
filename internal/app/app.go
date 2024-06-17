package app

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

var Version = "1.0.0"
var UninstallKey = "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Multiblox"
var ConfigKey = "SOFTWARE\\Intelblox Foundation\\Multiblox"

func Directory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	installDir := filepath.Join(homeDir, "AppData", "Local", "Multiblox")
	return installDir, nil
}

func EstimatedSize() (uint32, error) {
	appDir, err := Directory()
	if err != nil {
		return 0, err
	}
	var estimatedSize uint32 = 0
	filepath.Walk(appDir, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			estimatedSize += uint32(info.Size())
		}
		return nil
	})
	estimatedSize = estimatedSize / 1000
	return estimatedSize, nil
}

func Admin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func RunSelfAsAdmin(args ...string) error {
	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		fmt.Printf("Error converting string to pointer: %s\n", err)
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting exe path: %s\n", err)
		return err
	}
	exePtr, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		fmt.Printf("Error converting exe to pointer: %s\n", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting work directory: %s\n", err)
		return err
	}
	cwdPtr, err := syscall.UTF16PtrFromString(cwd)
	if err != nil {
		fmt.Printf("Error converting cwd to pointer: %s\n", err)
		return err
	}
	argsSlice := []string{}
	if len(os.Args) > 1 {
		argsSlice = os.Args[1:]
	}
	argsSlice = append(argsSlice, args...)
	argsStr := strings.Join(argsSlice, " ")
	argsPtr, err := syscall.UTF16PtrFromString(argsStr)
	if err != nil {
		fmt.Printf("Error converting args to pointer: %s\n", err)
	}
	err = windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, 1)
	if err != nil {
		fmt.Printf("Error running as administrator: %s\n", err)
		return err
	}
	return nil
}

func Ask(question string, defaultAnswer string, otherAnswers ...string) string {
	defaultAnswer = strings.ToLower(defaultAnswer)
	for {
		fmt.Print(question)
		choiceBytes, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
		if err != nil {
			fmt.Printf("Cannot read input: %s\n", err)
			return defaultAnswer
		}
		choice := strings.ToLower(string(choiceBytes))
		choice = choice[:len(choice)-2]
		if choice == "" || choice == defaultAnswer {
			return defaultAnswer
		}
		for _, answer := range otherAnswers {
			if choice == strings.ToLower(answer) {
				return choice
			}
		}
	}
}
