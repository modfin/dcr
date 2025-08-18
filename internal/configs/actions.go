package configs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type ActionItem struct {
	Action      string `yaml:"action"`
	Description string `yaml:"description"`
}

type ActionTemplates = map[string]ActionItem

type ActionItemWithType struct {
	//ActionItem
	Action      string `yaml:"action"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
}

type ActionGroup struct {
	Enabled bool                 `yaml:"enabled"`
	Verbose bool                 `yaml:"verbose"`
	Filter  string               `yaml:"filter"`
	Actions []ActionItemWithType `yaml:"actions"`
}

func RunActions(actionItemsWithType []ActionItemWithType, templates ActionTemplates, serviceName string, verbose bool) error {
	for _, actionItemWithType := range actionItemsWithType {
		executableActionItem, err := translateToExecutableActionItem(actionItemWithType, templates, serviceName)
		if err != nil {
			return fmt.Errorf("error getting effective action: %v", err)
		}
		err = runAction(executableActionItem.Action, verbose)
		if err != nil {
			return fmt.Errorf("error running action '%s': %v", executableActionItem, err)
		}
	}

	return nil
}

func translateToExecutableActionItem(actionItem ActionItemWithType, templates ActionTemplates, serviceName string) (*ActionItem, error) {
	if actionItem.Action == "" {
		return nil, fmt.Errorf("action item must either have a non-empty 'action' field")
	}

	var executableActionItem *ActionItem
	switch actionItem.Type {
	case "template":
		template, ok := templates[actionItem.Action]
		if !ok {
			return nil, fmt.Errorf("action template '%s' not found in action templates", actionItem.Action)
		}

		executableActionItem = &template
	default:
		executableActionItem = &ActionItem{actionItem.Action, actionItem.Description}
	}

	if strings.Contains(executableActionItem.Action, "%s") {
		executableActionItem.Action = fmt.Sprintf(executableActionItem.Action, serviceName)
	}

	return executableActionItem, nil
}

func runAction(action string, verbose bool) error {
	cmd := exec.Command("sh", "-c", action)

	var stdout bytes.Buffer
	if verbose {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	} else {
		cmd.Stdout = &stdout
	}
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil && !strings.Contains(stdout.String(), "does not exist in overrides") {
		return fmt.Errorf("Error running command '%s': %v\n", action, err)
	}

	return nil
}

func IsPreStart(args []string) bool {
	for _, arg := range args {
		if arg == "up" {
			return true
			break
		}
	}

	return false
}
