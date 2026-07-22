# Developer -> Engineer

## Go basis

### Packages in GO

- Unlike node there is no registry to pull the packages from, Go uses github to pull and store those in local /pkg path ot `GOPATH`.
- Everytime a project installs package by running `go` commands local /pkg is searched github is a fallback and a common place to store packages.
- By defining the modules inside `internal/` these cannot be accessed by other users Go handles this by default.