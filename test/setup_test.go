package test

import (
	"fmt"
	"os"
	"testing"
)

// Test configuration comes from the environment so no live credential is ever
// committed to this repository. Copy .env.example, fill it in, and export it
// before running the integration tests:
//
//	set -a && source .env && set +a && go test ./test/...
var (
	testJWTSecret     = os.Getenv("TEST_JWT_SECRET")
	testEncryptionKey = os.Getenv("TEST_ENCRYPTION_KEY")
	testDBURL         = os.Getenv("TEST_DB_URL")
)

// TestMain skips the whole package when the integration environment is absent,
// so a fresh clone can run `go test ./...` without a database.
func TestMain(m *testing.M) {
	if testDBURL == "" || testJWTSecret == "" || testEncryptionKey == "" {
		fmt.Println("skipping integration tests: TEST_DB_URL, TEST_JWT_SECRET and TEST_ENCRYPTION_KEY are not set")
		os.Exit(0)
	}
	os.Exit(m.Run())
}
