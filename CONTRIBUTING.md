# Contributing to Mako

We appreciate your interest in improving Mako. As an open-source, developer-focused project, we strive to maintain high engineering standards.

Please review the following guidelines before contributing code or documentation.

---

## Development Setup

### Prerequisites
* **Go**: Version 1.22 or higher.
* **Claude Code**: The `claude` command-line executable must be installed and available on your system's `PATH` if you wish to test the subprocess agent integration.
* **System Keychain**: An active system keyring (such as `gnome-keyring` or `dbus` on Linux, Keychain Access on macOS, or Credential Manager on Windows).

### Clone the Repository
```bash
git clone https://github.com/thenameisvicky/tokenly-mako.git
cd tokenly-mako
```

---

## Building and Running

### Build from Source
Compile the main executable to the root directory:
```bash
go build -o mako ./cmd/mako
```

### Run the Interactive Shell
Execute the compiled binary to start the REPL:
```bash
./mako
```

### Running Tests
Execute the unit tests:
```bash
go test -v ./...
```

---

## Coding Conventions

We adhere to standard Go guidelines and styling principles:
* **Formatting**: All Go files must be formatted using `gofmt`. We recommend running `go fmt ./...` before submitting any changes.
* **Linting**: Ensure your code is free of vet warnings by running:
  ```bash
  go vet ./...
  ```
* **Architecture**: Keep packages decoupled. System keychain interaction belongs in `internal/keychain`, and process launching belongs in `internal/agent_runtime`.
* **Errors**: Wrap sentinel and runtime errors descriptive of their operational context using `%w`.

---

## Commit Style

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for clear git histories:

* **Format**: `<type>(<scope>): <description>`
* **Types**:
  * `feat`: A new user-facing capability.
  * `fix`: A bug fix.
  * `docs`: Documentation updates.
  * `style`: Code style improvements (formatting, missing semicolons, etc. without logic changes).
  * `refactor`: Code changes that neither fix bugs nor add features.
  * `test`: Adding or correcting tests.
  * `chore`: Build steps, dependency updates, or auxiliary tool configuration.

**Example**:
```text
feat(shell): add auto-complete bindings for /status command
```

---

## Pull Requests

1. **Branch Naming**: Use descriptive branch names like `feat/interactive-completions` or `fix/keychain-fallback`.
2. **One Change per PR**: Keep Pull Requests focused on a single logical feature or issue fix.
3. **Write Tests**: Provide unit tests for any new logic added.
4. **Documentation**: Update the `README.md` and slash command lists if you modify, add, or deprecate user interface commands.
5. **Clean History**: Rebase your branch against `main` before submitting to avoid unnecessary merge conflicts.
