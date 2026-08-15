# PR Description: [TYPE] [PR-XXX] Short Title

<!-- 
PR Title Format: [TYPE] [PR-XXX] Short Description
Example: [FEAT] [PR-001] Merge CLI and Gateway into monorepo
Types: FEAT, REF, FIX, CHORE
-->

## Description
Please include a summary of the changes and the related issue/feature/refactor ticket.

## PR Number & Tracking
- PR Number: #XXX (Three-digit running counter starting from 000; e.g. 001)

## Type of Change
- [ ] [FEAT] New feature (non-breaking change which adds functionality)
- [ ] [REF] Refactoring (code reorganization, no behavior changes)
- [ ] [FIX] Bug fix (non-breaking change which fixes an issue)
- [ ] [CHORE] Maintenance, dependency updates, configuration

---

## Detailed Changes

### 1. Reorganization & Added Code
*Provide a high-level list of new directories, files, or structures.*

### 2. Modified Packages (`internal/` / `pkg/` / `cmd/`)
*List major packages modified and why.*

### 3. Dependency Alignment & Configuration
*List library changes, configuration file changes, etc.*

---

## Test Cases Tested

Provide details of the verification steps performed.

### 1. Workspace Unit & Integration Tests
*Include test command and whether it succeeded (e.g. `go test ./...`).*

### 2. Compilation Verification
*List built binaries and their paths (e.g. `go build -o bin/gateway ./cmd/gateway`).*

### 3. Manual Testing / Scenario Verifications
*Detail manual checks, API responses, or command executions tested.*
