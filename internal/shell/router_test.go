package shell

import (
	"errors"
	"testing"
)

type mockHandler struct {
	called bool
	args   []string
	err    error
}

func (m *mockHandler) Execute(args []string) error {
	m.called = true
	m.args = args
	return m.err
}

func TestRouter_RegisterAndRoute(t *testing.T) {
	router := NewRouter()

	handler := &mockHandler{}
	router.RegisterCommand("/test", "Test command description", handler)

	if !router.HasCommand("/test") {
		t.Error("expected router to have command /test")
	}

	if router.HasCommand("/unknown") {
		t.Error("expected router to not have command /unknown")
	}

	// Test successful route execution
	args := []string{"arg1", "arg2"}
	err := router.Route("/test", args)
	if err != nil {
		t.Errorf("unexpected error on routing: %v", err)
	}

	if !handler.called {
		t.Error("expected mockHandler.Execute to be called")
	}

	if len(handler.args) != 2 || handler.args[0] != "arg1" || handler.args[1] != "arg2" {
		t.Errorf("expected handler args to be %v, got %v", args, handler.args)
	}
}

func TestRouter_RouteNotFound(t *testing.T) {
	router := NewRouter()

	err := router.Route("/unknown", nil)
	if err == nil {
		t.Error("expected routing to unknown command to return an error")
	}
}

func TestRouter_CommandsOrder(t *testing.T) {
	router := NewRouter()

	router.RegisterCommand("/first", "First", &mockHandler{})
	router.RegisterCommand("/second", "Second", &mockHandler{})
	router.RegisterCommand("/third", "Third", &mockHandler{})

	cmds := router.Commands()
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}

	expectedOrder := []string{"/first", "/second", "/third"}
	for i, name := range expectedOrder {
		if cmds[i].Name != name {
			t.Errorf("expected command at index %d to be %q, got %q", i, name, cmds[i].Name)
		}
	}
}

func TestRouter_RouteExit(t *testing.T) {
	router := NewRouter()
	router.RegisterCommand("/exit", "Exit", CommandHandlerFunc(func(args []string) error {
		return ErrExit
	}))

	err := router.Route("/exit", nil)
	if !errors.Is(err, ErrExit) {
		t.Errorf("expected ErrExit, got: %v", err)
	}
}
