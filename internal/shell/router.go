package shell

import (
	"errors"
	"fmt"
)

var ErrExit = errors.New("exit shell")

type CommandHandler interface {
	Execute(args []string) error
}

type CommandHandlerFunc func(args []string) error

func (f CommandHandlerFunc) Execute(args []string) error {
	return f(args)
}

type CommandInfo struct {
	Name        string
	Description string
	Handler     CommandHandler
}

type Router struct {
	commands map[string]CommandInfo
	order    []string
}

func NewRouter() *Router {
	return &Router{
		commands: make(map[string]CommandInfo),
		order:    make([]string, 0),
	}
}

func (r *Router) RegisterCommand(name, description string, handler CommandHandler) {
	if _, exists := r.commands[name]; !exists {
		r.order = append(r.order, name)
	}
	r.commands[name] = CommandInfo{
		Name:        name,
		Description: description,
		Handler:     handler,
	}
}

func (r *Router) Route(name string, args []string) error {
	cmd, exists := r.commands[name]
	if !exists {
		return fmt.Errorf("command not found: %s", name)
	}
	return cmd.Handler.Execute(args)
}

func (r *Router) HasCommand(name string) bool {
	_, exists := r.commands[name]
	return exists
}

func (r *Router) Commands() []CommandInfo {
	cmds := make([]CommandInfo, 0, len(r.order))
	for _, name := range r.order {
		if cmd, exists := r.commands[name]; exists {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}