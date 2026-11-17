package api

import (
	"fmt"
	"slices"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// CheckPaths refuses a bespoke route claiming a path the registry claims, and two bespoke routes
// claiming one path under one method, so no route is served by two definitions (docs/UI.md section
// 17.1). The contract generator and the read API's mount both run it over the bespoke routes of reads,
// setup and policymanagement.
func CheckPaths(registryPaths []string, bespoke []server.Route) error {
	seen := map[string]string{}
	for _, route := range bespoke {
		if slices.Contains(registryPaths, route.Pattern) {
			return fmt.Errorf("the path %s is claimed by both the registry and the bespoke handler %s", route.Pattern, route.Operation)
		}
		key := route.Verb() + " " + route.Pattern
		if prior, ok := seen[key]; ok {
			return fmt.Errorf("the path %s is claimed by two bespoke handlers, %s and %s", key, prior, route.Operation)
		}
		seen[key] = route.Operation
	}
	return nil
}
