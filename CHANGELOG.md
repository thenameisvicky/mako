# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0-alpha] - 26-07-2026

### Added
- **Interactive Go Shell**: Low-latency terminal REPL using raw console mode with history, command routing, and basic autocompletion.
- **Secure Keychain Storage**: Platform-agnostic integration utilizing native credential vaults (macOS Keychain, Linux Secret Service/DBus, and Windows Credential Manager) via `go-keyring`.
- **Downstream Agent Execution**: Process runner to fork and execute `claude` (Claude Code) as a child process.
- **Native Terminal Pass-Through**: Direct piping of `stdin`, `stdout`, and `stderr` streams to preserve colors, input captures, and interactive prompts.
- **Slash Commands**: Initial set of terminal utilities:
  - `/init` to configure Tokenly backend credentials.
  - `/status` to display the active keychain credential state.
  - `/claude` to locate and launch the Claude Code sub-shell.
  - `/logout` to clear credentials.
  - `/clear` and `/clean` to clear terminal output.
  - `/help` and `/exit` to manage the REPL session.
