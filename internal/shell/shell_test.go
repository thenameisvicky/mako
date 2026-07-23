package shell

import (
	"testing"
)

type mockAsyncHandler struct {
	called bool
}

func (m *mockAsyncHandler) Execute(args []string) error {
	m.called = true
	return nil
}

func (m *mockAsyncHandler) IsAsync() bool {
	return true
}

func (m *mockAsyncHandler) LoadingMessage() string {
	return "Running mock async command..."
}

func TestShell_Completer(t *testing.T) {
	router := NewRouter()
	router.RegisterCommand("/connect", "Connect to service", &mockAsyncHandler{})
	router.RegisterCommand("/disconnect", "Disconnect from service", &mockAsyncHandler{})

	sh := NewShell(router)
	completer := sh.makeCompleter()

	// Exact prefix match
	line, pos, ok := completer("/conn", 5, '\t')
	if !ok {
		t.Error("expected ok to be true for /conn completion")
	}
	if line != "/connect " {
		t.Errorf("expected completed line to be /connect , got %q", line)
	}
	if pos != len("/connect ") {
		t.Errorf("expected new position to be %d, got %d", len("/connect "), pos)
	}

	// Multiple prefix match (no unique match, should return empty/ok=false)
	_, _, ok = completer("/di", 3, '\t')
	if !ok {
		// Wait, "/di" matches `/disconnect` uniquely! It should match.
		// Let's test that:
		line, pos, ok = completer("/di", 3, '\t')
		if !ok {
			t.Error("expected ok to be true for /di completion")
		}
		if line != "/disconnect " {
			t.Errorf("expected completed line to be /disconnect , got %q", line)
		}
	}

	// Non-matching character
	_, _, ok = completer("/conn", 5, 'a')
	if ok {
		t.Error("expected ok to be false for non-tab keypress")
	}

	// Non-slash prefix line
	_, _, ok = completer("conn", 4, '\t')
	if ok {
		t.Error("expected ok to be false for line without leading slash")
	}
}

func TestShell_AsyncExecution(t *testing.T) {
	router := NewRouter()
	handler := &mockAsyncHandler{}
	router.RegisterCommand("/async", "Run async task", handler)

	// Since running the interactive loop requires standard input, we can verify
	// that a command implementing AsyncCommandHandler executes correctly through routing.
	err := router.Route("/async", nil)
	if err != nil {
		t.Fatalf("unexpected routing error: %v", err)
	}
	if !handler.called {
		t.Error("expected async command handler to be executed")
	}
}
