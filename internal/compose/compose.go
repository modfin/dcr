package compose

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/modfin/dcr/internal/groups"
	"github.com/modfin/dcr/utils"
)

type Compose struct {
	Services map[string]Service `yaml:"services"`
}

type Service = map[string]interface{}

func FindDependencies(services []string, composeObj Compose) ([]string, error) {
	allDependencies := make([]string, 0)

	for _, serviceName := range services {
		services := composeObj.Services

		service := services[serviceName]

		dependsOnValue, ok := service["depends_on"]
		if !ok {
			continue // has no dependencies
		}

		depsInterface, ok := dependsOnValue.([]interface{})
		if !ok {
			return nil, fmt.Errorf("service '%s' 'depends_on' field is not a list", service)
		}

		for _, dep := range depsInterface {
			depStr, ok := dep.(string)
			if !ok {
				return nil, fmt.Errorf("service '%s' 'depends_on' field contains non-string value", service)
			}

			allDependencies = append(allDependencies, depStr)
		}

	}

	return allDependencies, nil
}

func GetServicesToBeStarted(args []string, composeObj Compose, groupObj *groups.Groups) []string {
	groupServices := utils.Uniq(groups.GetGroupServices(args, groupObj))
	servicesToBeStarted := make([]string, 0)
	allServices := GetServices(composeObj)

	for _, service := range groupServices {
		for _, arg := range allServices {
			if arg == service {
				servicesToBeStarted = append(servicesToBeStarted, service)
			}
		}
	}

	return servicesToBeStarted
}

func GetServices(composeObj Compose) []string {
	services := composeObj.Services
	keys := make([]string, 0, 1)

	for k, _ := range services {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func GetFilteredServices(services []string, filterRegex string, ignoredServices []string) []string {
	filteredServices := make([]string, 0)

	for _, dep := range services {
		shouldBeIgnored := false

		for _, ignore := range ignoredServices {
			if dep == ignore {
				shouldBeIgnored = true
				break
			}
		}

		if !shouldBeIgnored && regexp.MustCompile(filterRegex).MatchString(dep) {
			filteredServices = append(filteredServices, dep)
		}
	}

	return filteredServices
}
