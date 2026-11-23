// Package keychain stores the CLI's credentials.
//
// The OS keychain is used where one is available. On a headless machine - a
// server, a container, a root shell with no desktop session - Linux has no
// Secret Service to talk to, and the keyring calls fail with a D-Bus error. So
// that mako still works there, the package falls back to a file in the user's
// home directory with 0600 permissions.
//
// The fallback is protected by file permissions, not by encryption: there is
// nowhere to keep a key that an attacker who can read the file could not also
// read. That is the same trade-off the GitHub and Docker CLIs make, and
// CredentialsBackend reports which one is in use so it is never a surprise.
package keychain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

const (
	Service  = "mako"
	EmailKey = "email"
	TokenKey = "token"

	// ProviderKeyKey stores the user's OWN upstream provider API key.
	//
	// Mako does not swap credentials, so this key belongs to the user and is
	// forwarded to the provider untouched. It never reaches the proxy.
	ProviderKeyKey = "provider_key"

	// probeKey is read to decide whether a Secret Service is reachable. It is
	// never written, so the probe cannot disturb anything already stored.
	probeKey = "__backend_probe__"
)

// Backend names, also accepted in MAKO_CREDENTIALS_BACKEND.
const (
	BackendKeyring = "keyring"
	BackendFile    = "file"
)

var (
	resolveOnce sync.Once
	resolved    store
	resolvedID  string
)

type store interface {
	set(key, value string) error
	get(key string) (string, error)
	remove(key string) error
}

// backend returns the credential store for this machine, deciding once.
func backend() store {
	resolveOnce.Do(func() {
		switch os.Getenv("MAKO_CREDENTIALS_BACKEND") {
		case BackendFile:
			resolved, resolvedID = fileStore{}, BackendFile
			return
		case BackendKeyring:
			resolved, resolvedID = keyringStore{}, BackendKeyring
			return
		}

		// Auto: a reachable Secret Service either returns the probe value or
		// reports that it is simply not set. Any other error means no service.
		_, err := keyring.Get(Service, probeKey)
		if err == nil || errors.Is(err, keyring.ErrNotFound) {
			resolved, resolvedID = keyringStore{}, BackendKeyring
			return
		}
		resolved, resolvedID = fileStore{}, BackendFile
	})
	return resolved
}

// CredentialsBackend reports where credentials are being stored, for /status
// and for the warning shown after a login that could not use the OS keychain.
func CredentialsBackend() string {
	backend()
	return resolvedID
}

// UsingFileFallback reports whether credentials live in a plain file because no
// OS keychain was available.
func UsingFileFallback() bool { return CredentialsBackend() == BackendFile }

// --- keyring backend ---------------------------------------------------------

type keyringStore struct{}

func (keyringStore) set(key, value string) error { return keyring.Set(Service, key, value) }
func (keyringStore) get(key string) (string, error) {
	return keyring.Get(Service, key)
}
func (keyringStore) remove(key string) error { return keyring.Delete(Service, key) }

// --- file backend ------------------------------------------------------------

type fileStore struct{}

// CredentialsPath returns the file the fallback backend uses.
func CredentialsPath() (string, error) {
	if v := os.Getenv("MAKO_CREDENTIALS_FILE"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to locate home directory: %w", err)
	}
	return filepath.Join(home, ".mako", "credentials.json"), nil
}

func (fileStore) load() (map[string]string, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unable to read %s: %w", path, err)
	}

	values := map[string]string{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("credentials file %s is corrupt: %w", path, err)
	}
	return values, nil
}

func (f fileStore) save(values map[string]string) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("unable to create %s: %w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}

	// Write then rename, so a crash cannot leave a half-written file, and create
	// with 0600 so the token is never briefly world-readable.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("unable to write %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("unable to update %s: %w", path, err)
	}
	return nil
}

func (f fileStore) set(key, value string) error {
	values, err := f.load()
	if err != nil {
		return err
	}
	values[key] = value
	return f.save(values)
}

func (f fileStore) get(key string) (string, error) {
	values, err := f.load()
	if err != nil {
		return "", err
	}
	v, ok := values[key]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (f fileStore) remove(key string) error {
	values, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := values[key]; !ok {
		return keyring.ErrNotFound
	}
	delete(values, key)
	return f.save(values)
}

// --- public API --------------------------------------------------------------

// Save stores the account identity and Mako token
func Save(email string, token string) error {
	s := backend()
	if err := s.set(EmailKey, email); err != nil {
		return fmt.Errorf("unable to store credentials: %w", err)
	}
	if err := s.set(TokenKey, token); err != nil {
		return fmt.Errorf("unable to store credentials: %w", err)
	}
	return nil
}

// Load returns the stored identity and Mako token
func Load() (string, string, error) {
	s := backend()
	email, err := s.get(EmailKey)
	if err != nil {
		return "", "", err
	}
	token, err := s.get(TokenKey)
	if err != nil {
		return "", "", err
	}
	return email, token, nil
}

// Exists reports whether an account is stored
func Exists() bool {
	_, err := backend().get(EmailKey)
	return err == nil
}

// Delete clears every stored credential, ignoring entries that are already gone
func Delete() error {
	s := backend()
	for _, key := range []string{EmailKey, TokenKey, ProviderKeyKey} {
		if err := s.remove(key); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
	}
	return nil
}

// SaveProviderKey stores the user's own provider API key
func SaveProviderKey(apiKey string) error {
	if err := backend().set(ProviderKeyKey, apiKey); err != nil {
		return fmt.Errorf("unable to store the provider key: %w", err)
	}
	return nil
}

// LoadProviderKey reads the user's own provider API key
func LoadProviderKey() (string, error) { return backend().get(ProviderKeyKey) }

// DeleteProviderKey removes the stored provider API key, ignoring a missing one
func DeleteProviderKey() error {
	if err := backend().remove(ProviderKeyKey); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}
