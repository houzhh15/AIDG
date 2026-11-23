package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

type registeredRoute struct {
	method string
	path   string
}

func literalAPIRoutesInMain(t *testing.T) []registeredRoute {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	unique := map[registeredRoute]struct{}{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !methods[selector.Sel.Name] {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		path, err := strconv.Unquote(literal.Value)
		if err != nil || (!strings.HasPrefix(path, "/api/v1/") && !strings.HasPrefix(path, "/internal/api/v1/")) {
			return true
		}
		unique[registeredRoute{method: selector.Sel.Name, path: path}] = struct{}{}
		return true
	})
	routes := make([]registeredRoute, 0, len(unique))
	for route := range unique {
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].path == routes[j].path {
			return routes[i].method < routes[j].method
		}
		return routes[i].path < routes[j].path
	})
	return routes
}

func TestRegisteredAPIRoutesHaveAuthorizationClassification(t *testing.T) {
	scopes := configuredRouteScopes()
	authenticatedOnly := authenticatedOnlyRouteKeys()
	var missing []string
	for _, route := range literalAPIRoutesInMain(t) {
		key := route.method + " " + route.path
		if isExplicitPublicRoute(route.method, route.path) {
			continue
		}
		if _, ok := authenticatedOnly[key]; ok {
			continue
		}
		if _, ok := configuredScopesForRoute(scopes, route.method, route.path); ok {
			continue
		}
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		t.Fatalf("registered API routes without public/authenticated/scope classification:\n%s", strings.Join(missing, "\n"))
	}
}

func TestProjectAndUserListsAreExplicitlyAuthenticatedOnly(t *testing.T) {
	authenticatedOnly := authenticatedOnlyRouteKeys()
	for _, key := range []string{"GET /api/v1/projects", "GET /api/v1/users"} {
		if _, ok := authenticatedOnly[key]; !ok {
			t.Fatalf("%s must be explicitly authenticated-only", key)
		}
		if _, ok := configuredRouteScopes()[key]; ok {
			t.Fatalf("%s must not acquire an unrelated scope", key)
		}
	}
}

func TestIdentityProviderRoutesUseDedicatedScopes(t *testing.T) {
	scopes := configuredRouteScopes()
	for _, key := range []string{
		"GET /api/v1/identity-providers",
		"GET /api/v1/identity-providers/:id",
	} {
		if got := scopes[key]; len(got) != 1 || got[0] != users.ScopeIdpRead {
			t.Fatalf("%s scopes=%v", key, got)
		}
	}
	for _, key := range []string{
		"POST /api/v1/identity-providers",
		"PUT /api/v1/identity-providers/:id",
		"DELETE /api/v1/identity-providers/:id",
		"POST /api/v1/identity-providers/:id/test",
		"POST /api/v1/identity-providers/:id/sync",
	} {
		if got := scopes[key]; len(got) != 1 || got[0] != users.ScopeIdpWrite {
			t.Fatalf("%s scopes=%v", key, got)
		}
	}
}

func TestMainRouteLiteralsDoNotContainMissingParameterSlash(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{"/api/v1/users:username", "/api/v1/users:id", "/api/v1/projects:id"} {
		if strings.Contains(string(data), malformed) {
			t.Fatalf("malformed route literal remains: %s", malformed)
		}
	}
}
