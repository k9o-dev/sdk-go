package k9o

import (
	"strings"
	"testing"
)

func TestKErrorError(t *testing.T) {
	e := &KError{Code: "K9O_TIER_CAP_HIT", Message: "cap exceeded", Status: 429}
	msg := e.Error()
	if !strings.Contains(msg, "K9O_TIER_CAP_HIT") {
		t.Errorf("Error() missing code: %q", msg)
	}
	if !strings.Contains(msg, "cap exceeded") {
		t.Errorf("Error() missing message: %q", msg)
	}
	if !strings.Contains(msg, "429") {
		t.Errorf("Error() missing status: %q", msg)
	}
}

func TestKErrorErrorNoStatus(t *testing.T) {
	e := &KError{Code: "K9O_INVALID_BASE_URL", Message: "bad url"}
	msg := e.Error()
	if strings.Contains(msg, "status=") {
		t.Errorf("Error() should not include status=0: %q", msg)
	}
}

func TestKErrorFromWire(t *testing.T) {
	wire := &KErrorWire{
		Code:           "K9O_PROJECT_NOT_FOUND",
		Message:        "no such project",
		Context:        map[string]any{"projectSlug": "ghost"},
		Recommendation: "create the project first",
		RequestID:      "req_abc",
	}
	e := kerrorFromWire(wire, 404)
	if e.Code != "K9O_PROJECT_NOT_FOUND" {
		t.Errorf("Code: got %q", e.Code)
	}
	if e.Status != 404 {
		t.Errorf("Status: got %d", e.Status)
	}
	if e.RequestID != "req_abc" {
		t.Errorf("RequestID: got %q", e.RequestID)
	}
	if e.Context["projectSlug"] != "ghost" {
		t.Errorf("Context: got %v", e.Context)
	}
}

func TestKErrorFromWireUnknownCodePassthrough(t *testing.T) {
	wire := &KErrorWire{Code: "K9O_NOVEL_CODE", Message: "y"}
	e := kerrorFromWire(wire, 422)
	if e.Code != "K9O_NOVEL_CODE" {
		t.Errorf("Code: got %q", e.Code)
	}
	if e.Status != 422 {
		t.Errorf("Status: got %d", e.Status)
	}
}
