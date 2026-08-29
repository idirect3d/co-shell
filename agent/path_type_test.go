package agent

import (
	"context"
	"strings"
	"testing"
)

// TestSearchPathType verifies searchFilesTool's behavior when args["path"] is
// missing, a float64 (as JSON numbers decode to), or a string. This isolates
// whether the "path argument is required" error is caused by a type mismatch
// in the OpenAI (JSON) mode.
func TestSearchPathType(t *testing.T) {
	a := &Agent{}

	// Case 1: path missing -> should error "path argument is required".
	_, err := a.searchFilesTool(context.Background(), map[string]interface{}{
		"regex": "foo",
	})
	if err == nil || !strings.Contains(err.Error(), "path argument is required") {
		t.Errorf("missing path: want 'path argument is required', got %v", err)
	}

	// Case 2: path is a float64 (JSON numbers decode to float64) -> should error.
	_, err = a.searchFilesTool(context.Background(), map[string]interface{}{
		"path":  float64(123),
		"regex": "foo",
	})
	if err == nil || !strings.Contains(err.Error(), "path argument is required") {
		t.Errorf("float64 path: want 'path argument is required', got %v", err)
	}

	// Case 3: path is a string -> should NOT error on the path assertion.
	// It may error later (e.g. invalid regex or dir not found), but not with
	// "path argument is required".
	_, err = a.searchFilesTool(context.Background(), map[string]interface{}{
		"path":  "/tmp",
		"regex": "foo",
	})
	if err != nil && strings.Contains(err.Error(), "path argument is required") {
		t.Errorf("string path: should not report 'path argument is required', got %v", err)
	}
}
