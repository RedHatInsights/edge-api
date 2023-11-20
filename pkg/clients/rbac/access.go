package rbac

import "strings"

const permissionDelimiter = ":"

// AccessList is a slice of Accesses and is generally used to represent a principal's
// full set of permissions for an application
type AccessList []Access

// Access represents a permission and an optional resource definition
type Access struct {
	ResourceDefinitions []ResourceDefinition `json:"resourceDefinitions,omitempty"`
	Permission          string               `json:"permission"`
}

// ResourceDefinition limits an Access to specific resources
type ResourceDefinition struct {
	Filter ResourceDefinitionFilter `json:"attributeFilter"`
}

// ResourceDefinitionFilter represents the key/values used for filtering
type ResourceDefinitionFilter struct {
	Key       string    `json:"key"`
	Operation string    `json:"operation"`
	Value     []*string `json:"value"`
}

// Application returns the name of the application in the permission
func (a Access) Application() string {
	return permIndex(a.Permission, 0)
}

// Resource returns the name of the resource in the permission
func (a Access) Resource() string {
	return permIndex(a.Permission, 1)
}

// AccessType returns the access type in the permission
func (a Access) AccessType() string {
	return permIndex(a.Permission, 2)
}

// permIndex return the permission item value at index when splitting by permission delimiter
// the permission looks like "inventory:hosts:read" where:
// inventory is the application name and locate at index 0
// hosts is the resource and located at index 1
// read is the access type and located at index 2
func permIndex(permission string, index int) string {
	permissionItems := strings.Split(permission, permissionDelimiter)
	// a correct permission must have 3 items application:resource:access-type
	if len(permissionItems) == 3 {
		return permissionItems[index]
	}
	return ""
}
