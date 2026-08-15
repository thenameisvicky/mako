package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenameisvicky/tokenly-gateway/internal/shell"
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
	router := shell.NewRouter()
	router.RegisterCommand("/connect", "Connect to service", &mockAsyncHandler{})
	router.RegisterCommand("/disconnect", "Disconnect from service", &mockAsyncHandler{})

	sh := shell.NewShell(router)
	completer := sh.MakeCompleter()

	// Exact prefix match
	line, pos, ok := completer("/conn", 5, '\t')
	assert.True(t, ok)
	assert.Equal(t, "/connect ", line)
	assert.Equal(t, len("/connect "), pos)

	// Multiple prefix match
	line, pos, ok = completer("/di", 3, '\t')
	assert.True(t, ok)
	assert.Equal(t, "/disconnect ", line)

	// Non-matching character
	_, _, ok = completer("/conn", 5, 'a')
	assert.False(t, ok)

	// Non-slash prefix line
	_, _, ok = completer("conn", 4, '\t')
	assert.False(t, ok)
}

func TestShell_AsyncExecution(t *testing.T) {
	router := shell.NewRouter()
	handler := &mockAsyncHandler{}
	router.RegisterCommand("/async", "Run async task", handler)

	err := router.Route("/async", nil)
	assert.NoError(t, err)
	assert.True(t, handler.called)
}
