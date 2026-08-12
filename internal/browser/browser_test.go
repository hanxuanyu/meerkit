package browser

import (
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

func TestManagerRejectsMissingExtension(t *testing.T) {
	manager, err := NewManager("secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.ExecuteAction(t.Context(), sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 1}, Action: sdk.BrowserAction{Type: "page.wait"}})
	if err == nil {
		t.Fatal("expected unavailable extension error")
	}
}

func TestTargetsRejectMissingExtension(t *testing.T) {
	manager, err := NewManager("secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Targets(t.Context(), "agent")
	if err == nil {
		t.Fatal("expected unavailable extension error")
	}
}
