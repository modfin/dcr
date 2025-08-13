package zdap

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/modfin/dcr/internal/compose"
	"github.com/modfin/dcr/internal/configs"
	"github.com/modfin/dcr/internal/groups"
	"github.com/modfin/dcr/utils"
)

var zdap configs.Zdap

func RunPreStartActions(config *configs.Config, args []string, composeObj compose.Compose, groupObj *groups.Groups) error {
	if !utils.IsInstalled("zdap") {
		fmt.Println("Zdap integration is enabled, but zdap is not installed. Please install https://github.com/modfin/zdap to use this feature.")
		return nil
	}
	zdap = config.Integrations.Zdap

	zdapDependencies, err := getZdapDependencies(args, composeObj, groupObj)
	if err != nil {
		return fmt.Errorf("error getting zdap dependencies: %v", err)
	}

	if len(zdapDependencies) == 0 {
		fmt.Println("No dependencies")
		return nil
	}

	if len(zdap.ResourceNameOverrides) > 0 {
		for _, override := range zdap.ResourceNameOverrides {
			if override.Name == "" || override.ResourceName == "" {
				return fmt.Errorf("resource name override must have both 'name' and 'resource-name' fields")
			}

			for i, dep := range zdapDependencies {
				if dep == override.Name {
					zdapDependencies[i] = override.ResourceName
				}
			}
		}
	}

	if zdap.OnlyUnclonedDependencies {
		fmt.Println("Looking for uncloned dependencies to determine which ones to clone...")
		servicesWithSnaps, err := getServicesWithSnapsMap()
		if err != nil {
			return fmt.Errorf("error finding services with snapshot dependencies: %v", err)
		}

		for _, name := range zdapDependencies {
			if hasSnap, ok := servicesWithSnaps[name]; ok && hasSnap {
				fmt.Printf("✅ Dependency '%s' already has an attached clone.\n", name)
			} else {
				fmt.Printf("❌ Dependency '%s' has no attached clone on any origin. Running actions...\n", name)

				if err := configs.RunActions(zdap.PreStartActions.Actions, zdap.ActionTemplates, name, zdap.PreStartActions.Verbose); err != nil {
					return fmt.Errorf("error running zdap actions for service '%s': %v", name, err)
				}
			}
		}

		return nil
	}

	for _, name := range zdapDependencies {
		fmt.Printf("Running zdap actions for service '%s'...\n", name)
		if err := configs.RunActions(config.Integrations.Zdap.PreStartActions.Actions, config.Integrations.Zdap.ActionTemplates, name, zdap.PreStartActions.Verbose); err != nil {
			return fmt.Errorf("error running zdap actions: %v", err)
		}
	}

	return nil
}

func getZdapDependencies(args []string, composeObj compose.Compose, groupObj *groups.Groups) ([]string, error) {
	servicesToBeStarted := compose.GetServicesToBeStarted(args, composeObj, groupObj)
	dependencies, err := compose.FindDependencies(servicesToBeStarted, composeObj)
	if err != nil {
		return nil, fmt.Errorf("error finding dependencies: %v", err)
	}

	return compose.GetFilteredServices(utils.Uniq(dependencies), zdap.PreStartActions.Filter, zdap.IgnoredResources), nil
}

func getServicesWithSnapsMap() (map[string]bool, error) {
	cmd := exec.Command("zdap", "list", "clones")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("could not get stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start command: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	currentService := ""
	servicesWithSnaps := make(map[string]bool)

	timestampRegex := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		isTimestamp := timestampRegex.MatchString(trimmedLine)
		if !isTimestamp && (strings.HasPrefix(trimmedLine, "├ ") || strings.HasPrefix(trimmedLine, "└ ")) {
			parts := strings.SplitN(trimmedLine, " ", 2)
			if len(parts) > 1 {
				currentService = parts[1]
			}
		}

		if isTimestamp {
			if currentService != "" {
				servicesWithSnaps[currentService] = true
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("command finished with an error: %w", err)
	}

	return servicesWithSnaps, nil
}
