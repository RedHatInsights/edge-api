package utility

import (
	"github.com/Unleash/unleash-client-go/v4"
	unleashContext "github.com/Unleash/unleash-client-go/v4/context"
	feature "github.com/redhatinsights/edge-api/unleash/features"
)

// EnforceEdgeGroups returns if the organization is enforced to use edge groups
func EnforceEdgeGroups(orgID string) bool {
	return feature.EnforceEdgeGroups.IsEnabled(unleash.WithContext(unleashContext.Context{UserId: orgID}))
}
