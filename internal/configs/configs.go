package configs

type Config struct {
	Integrations Integrations `yaml:"integrations"`
	ActionConfig ActionConfig `yaml:"action-config"`
}

type Integrations struct {
	Zdap Zdap `yaml:"zdap"`
}

type ActionConfig struct {
	PreStartActions ActionGroup           `yaml:"pre-start-actions"`
	ActionTemplates map[string]ActionItem `yaml:"action-templates"`
}

type Zdap struct {
	OnlyUnclonedDependencies bool                       `yaml:"only-uncloned-dependencies"`
	Filter                   string                     `yaml:"filter"`
	IgnoredResources         []string                   `yaml:"ignored-resources"`
	ResourceNameOverrides    []ZdapResourceNameOverride `yaml:"resource-name-overrides"`
	PreStartActions          ActionGroup                `yaml:"pre-start-actions"`
	ActionTemplates          map[string]ActionItem      `yaml:"action-templates"`
}

type ZdapResourceNameOverride struct {
	Name         string `yaml:"name"`
	ResourceName string `yaml:"resource-name"`
}
