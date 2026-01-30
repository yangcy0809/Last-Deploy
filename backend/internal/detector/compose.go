package detector

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]any `yaml:"services"`
}

func parseComposeServices(content []byte) ([]string, error) {
	var cfg composeFile
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}

	if len(cfg.Services) == 0 {
		return nil, nil
	}

	services := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		if name == "" {
			return nil, fmt.Errorf("invalid compose service name: empty")
		}
		services = append(services, name)
	}
	sort.Strings(services)
	return services, nil
}

