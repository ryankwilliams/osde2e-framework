package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// Run executes the os.exec command provided
func Run(command *exec.Cmd) (io.Writer, io.Writer, error) {
	var stdout, stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr

	// TODO: Configure tee output to file/buffer

	err := command.Start()
	if err != nil {
		return command.Stdout, command.Stderr, fmt.Errorf("failed to start command: %v", err)
	}

	err = command.Wait()
	if err != nil {
		return command.Stdout, command.Stderr, fmt.Errorf("failed to wait for command to finish: %v", err)
	}

	return command.Stdout, command.Stderr, nil
}

// ConvertJSONStringToMap converts a json string formatted to a map object
func ConvertJSONStringToMap(data io.Writer) (map[string]any, error) {
	var result map[string]any
	err := json.Unmarshal([]byte(fmt.Sprint(data)), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
