package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenameisvicky/tokenly-gateway/internal/shell"
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
	router := shell.NewRouter()

	handler := &mockHandler{}
	router.RegisterCommand("/test", "Test command description", handler)

	assert.True(t, router.HasCommand("/test"))
	assert.False(t, router.HasCommand("/unknown"))

	// Test successful route execution
	args := []string{"arg1", "arg2"}
	err := router.Route("/test", args)
	assert.NoError(t, err)
	assert.True(t, handler.called)
	assert.Equal(t, args, handler.args)
}

func TestRouter_RouteNotFound(t *testing.T) {
	router := shell.NewRouter()

	err := router.Route("/unknown", nil)
	assert.Error(t, err)
}

func TestRouter_CommandsOrder(t *testing.T) {
	router := shell.NewRouter()

	router.RegisterCommand("/first", "First", &mockHandler{})
	router.RegisterCommand("/second", "Second", &mockHandler{})
	router.RegisterCommand("/third", "Third", &mockHandler{})

	cmds := router.Commands()
	assert.Len(t, cmds, 3)

	expectedOrder := []string{"/first", "/second", "/third"}
	for i, name := range expectedOrder {
		assert.Equal(t, name, cmds[i].Name)
	}
}

func TestRouter_RouteExit(t *testing.T) {
	router := shell.NewRouter()
	router.RegisterCommand("/exit", "Exit", shell.CommandHandlerFunc(func(args []string) error {
		return shell.ErrExit
	}))

	err := router.Route("/exit", nil)
	assert.ErrorIs(t, err, shell.ErrExit)
}
