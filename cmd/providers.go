package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/gecko-iac/gecko/internal/core"
	"github.com/gecko-iac/gecko/internal/lang"
	fossprovider "github.com/gecko-iac/gecko/providers/foss"
)

// providerFactories maps provider names to their constructors.
var providerFactories = map[string]func(map[string]interface{}) core.Provider{
	"proxmox":    func(c map[string]interface{}) core.Provider { return fossprovider.NewProxmoxProvider(c) },
	"fly":        func(c map[string]interface{}) core.Provider { return fossprovider.NewFlyProvider(c) },
	"openstack":  func(c map[string]interface{}) core.Provider { return fossprovider.NewOpenStackProvider(c) },
	"hostinger":  func(c map[string]interface{}) core.Provider { return fossprovider.NewHostingerProvider(c) },
	"ubicloud":   func(c map[string]interface{}) core.Provider { return fossprovider.NewUbicloudProvider(c) },
	"opennebula": func(c map[string]interface{}) core.Provider { return fossprovider.NewOpenNebulaProvider(c) },
}

// registerProviders instantiates and configures every provider the project
// declares, registering them on the stack. Configure failures and unknown
// providers are returned as warnings rather than aborting, matching apply
// behavior.
func registerProviders(ctx context.Context, loaded *lang.LoadedStack) []string {
	var warnings []string
	for _, hint := range loaded.ProviderHints {
		factory, ok := providerFactories[strings.ToLower(hint.Name)]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Unknown provider %q — skipping", hint.Name))
			continue
		}
		p := factory(hint.Config)
		if err := p.Configure(ctx, hint.Config); err != nil {
			warnings = append(warnings, fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
		}
		loaded.Stack.RegisterProvider(p)
	}
	return warnings
}

// providerNameForResourceType extracts the provider name from a resource
// type like "proxmox:vm".
func providerNameForResourceType(t core.ResourceType) string {
	if i := strings.Index(string(t), ":"); i > 0 {
		return string(t)[:i]
	}
	return string(t)
}
