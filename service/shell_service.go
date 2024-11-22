package service

import (
	"fmt"
	"os/exec"
)

func ExecGoCommand(args []string) (*string, error) {
	command := exec.Command("go", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		fmt.Printf("ExecGoCommand error %v output %s\n", err, string(output))
		return nil, err
	}
	s := string(output)
	return &s, nil
}
