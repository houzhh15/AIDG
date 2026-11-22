package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type compatibilityContract struct {
	SchemaVersion  int    `json:"schema_version"`
	Classification string `json:"classification"`
	Routes         []struct {
		Method           string `json:"method"`
		Path             string `json:"path"`
		RequiredScope    string `json:"required_scope,omitempty"`
		ExpectedStatuses []int  `json:"expected_statuses"`
	} `json:"routes"`
}

func TestCompatLegacyAPIContractManifest(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "..", "..", "test", "compat", "contracts", "legacy-api-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var contract compatibilityContract
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		t.Fatal(err)
	}
	if contract.SchemaVersion != 1 || contract.Classification == "" || len(contract.Routes) < 6 {
		t.Fatalf("incomplete contract: %+v", contract)
	}
	seen := map[string]bool{}
	for _, route := range contract.Routes {
		if route.Method == "" || route.Path == "" || len(route.ExpectedStatuses) == 0 {
			t.Fatalf("invalid route: %+v", route)
		}
		key := route.Method + " " + route.Path
		if seen[key] {
			t.Fatalf("duplicate route: %s", key)
		}
		seen[key] = true
	}
}
