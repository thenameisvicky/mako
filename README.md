# Mako

Mako is Tokenly's authenticated runtime for AI coding agents.

It provides a secure execution environment where compatible agents authenticate using your Tokenly identity instead of managing credentials themselves.

Mako stores credentials using the operating system's secure keychain, launches supported agents as child processes, and transparently manages authenticated communication with Tokenly services.

## Architecture

The system is designed as a modular, local-first interactive shell and process runner written in Go. The architecture consists of several key components:

1. **CLI Entrypoint (`cmd/mako`)**:
   Responsible for starting the main application, configuring environment signals, and launching the interactive shell.

2. **Interactive Shell Engine (`internal/shell`)**:
   - Implements a responsive Read-Eval-Print Loop (REPL) using raw terminal modes.
   - Provides a custom slash-command router for internal routing.
   - Includes user experience elements such as auto-completion (tab-completion), terminal boundary detection for responsive boxes, and background spinner synchronization for asynchronous tasks.

3. **Authentication Module (`internal/auth`)**:
   Handles password prompt security using `golang.org/x/term` for secure inputs and coordinates checking active credentials on application launch.

4. **Secure Storage Handler (`internal/keychain`)**:
   Provides cross-platform access to the operating system's native secure keychain (via DBus/Secret Service on Linux, Keychain on macOS, and Windows Credential Manager on Windows) using the `go-keyring` abstraction layer.

5. **Agent Runtime Execution (`internal/agent_runtime`)**:
   - Locates external AI agent executables (such as the Claude CLI) using the system path.
   - Launches agent executables as child processes.
   - Directly attaches process input, output, and error streams (`stdin`, `stdout`, `stderr`) to preserve terminal state, colorization, and interactivity.

## Execution Flow

```mermaid
sequenceDiagram
    actor Dev as Developer
    participant Mako as Mako CLI (mako)
    participant KC as System Keychain (go-keyring)
    participant Agent as Agent CLI (child process)

    Dev->>Mako: Run "mako"
    activate Mako
    Mako->>KC: Query credentials (Exists)
    KC-->>Mako: Return presence status
    Mako-->>Dev: Print Welcome UI & active session state

    Dev->>Mako: /init
    Mako->>Dev: Prompt for email/password credentials
    Dev->>Mako: Enter credentials
    Mako->>KC: Save credentials securely
    KC-->>Mako: Success
    Mako-->>Dev: Print confirmation

    Dev->>Mako: /claude
    Mako->>Mako: Locate 'claude' executable in PATH
    Mako->>Agent: Fork/Exec 'claude' (attach stdin/stdout/stderr)
    activate Agent
    Dev<->>Agent: Interactive session (Claude Code)
    Dev->>Agent: Exit Claude
    deactivate Agent
    Agent-->>Mako: Process exit
    Mako-->>Dev: Resume Mako interactive shell
    deactivate Mako
```

## Usage Examples

Below are standard invocation commands and interactive shell operations.

### Configuration and Session Management
```bash
# Authenticate with Tokenly
mako login

# Remove credentials from the system keychain
mako logout
```

### Proxy Controller
```bash
# Start background auth & proxy agent
mako proxy start

# Stop the running proxy agent
mako proxy stop

# Query the status of the proxy daemon
mako proxy status
```

### System Diagnostics and Management
```bash
# Run environment diagnostics and dependency health checks
mako doctor

# Check for updates and install the latest release
mako update

# Print current Mako release version information
mako version
```

### Interactive Sub-Shell Commands
Once inside the interactive Mako terminal, you can direct runtime control to downstream AI coding agents:
```
/claude     - Launch Claude agent
/codex      - Launch Codex agent
```

## Credits

- **Vigneshwaran B** (thenameisvicky)