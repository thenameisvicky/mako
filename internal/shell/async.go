package shell

// asyncCommand marks a handler that should run behind a spinner, for work that
// makes network calls and can take more than a moment.
type asyncCommand struct {
	run     func(args []string) error
	loading string
}

func newAsyncCommand(loading string, run func(args []string) error) *asyncCommand {
	return &asyncCommand{run: run, loading: loading}
}

func (a *asyncCommand) Execute(args []string) error { return a.run(args) }
func (a *asyncCommand) IsAsync() bool               { return true }
func (a *asyncCommand) LoadingMessage() string      { return a.loading }
