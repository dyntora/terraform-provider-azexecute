package client

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// This same fixture is tested against the API's C# DTOs. Additive fields are
// allowed, but neither implementation may lose or rename existing wire values.
func TestApplicationWireContract(t *testing.T) {
	data, err := os.ReadFile("testdata/application-response.json")
	if err != nil {
		t.Fatal(err)
	}
	var response Application
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	roundtrip, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual any
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(roundtrip, &actual); err != nil {
		t.Fatal(err)
	}
	assertWireSubset(t, "$", expected, actual)
	// The submission and grant status are distinct contracts.
	if len(response.RequestedAPIPermissions) != 1 || len(response.APIPermissionRequests) != 1 || response.APIPermissionRequests[0].Status != "Pending" {
		t.Fatal("lost original permission submission or permission status")
	}
}

func assertWireSubset(t *testing.T, path string, expected, actual any) {
	t.Helper()
	switch expected := expected.(type) {
	case map[string]any:
		object, ok := actual.(map[string]any)
		if !ok {
			t.Fatalf("%s: expected object", path)
		}
		for name, value := range expected {
			actualValue, exists := object[name]
			if !exists {
				t.Fatalf("%s.%s: missing wire property", path, name)
			}
			assertWireSubset(t, path+"."+name, value, actualValue)
		}
	case []any:
		array, ok := actual.([]any)
		if !ok || len(expected) != len(array) {
			t.Fatalf("%s: collection changed", path)
		}
		for i, value := range expected {
			assertWireSubset(t, path+"[]", value, array[i])
		}
	default:
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("%s: wire value changed: expected %v, got %v", path, expected, actual)
		}
	}
}
