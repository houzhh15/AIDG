package main

import (
	"testing"

	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

func TestSyncRoutesAreCoveredByAuthorizationPolicy(t *testing.T) {
	if _, ok := authenticatedOnlyRouteKeys()["GET /api/v1/sync/capabilities"]; !ok {
		t.Fatal("sync capabilities route is not authentication-protected")
	}
	if _, ok := authenticatedOnlyRouteKeys()["GET /api/v1/sync/projects"]; !ok {
		t.Fatal("sync projects route is not authentication-protected")
	}
	tests := []struct {
		path     string
		required string
	}{
		{"/api/v1/sync/projects/project-1/manifest", users.ScopeTaskRead},
		{"/api/v1/sync/projects/project-1/documents/feature_list", users.ScopeProjectDocRead},
		{"/api/v1/sync/projects/project-1/tasks/task-1/documents/requirements", users.ScopeTaskRead},
	}
	for _, test := range tests {
		scopes, ok := configuredScopesForRoute(configuredRouteScopes(), "GET", test.path)
		if !ok {
			t.Fatalf("sync route %s is not classified", test.path)
		}
		found := false
		for _, scope := range scopes {
			if scope == test.required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("sync route %s does not require %s: %v", test.path, test.required, scopes)
		}
	}
}
