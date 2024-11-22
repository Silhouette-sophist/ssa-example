package service

import (
	"fmt"
	"os/exec"
	"strings"
)

func ExecGoCommandWithDir(execDir string, args []string) (string, error) {
	command := exec.Command("go", args...)
	command.Dir = execDir
	output, err := command.CombinedOutput()
	if err != nil {
		fmt.Printf("ExecGoCommandWithDir error %v output %s\n", err, string(output))
		return "", err
	}
	s := strings.TrimSuffix(string(output), "\n")
	return s, nil
}
