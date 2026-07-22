package keychain

import (
	"github.com/zalando/go-keyring" 
	"fmt"
)

const (
	Service = "tokenly"
	EmailKey = "email"
	TokenKey = "token"
)

func Save(email string, password string) error {
	if err := keyring.Set(Service, EmailKey, email); err != nil {
		return fmt.Errorf(
			"unable to access system keychain: %w",
			err,
		)
	}

	if err := keyring.Set(Service, TokenKey, password); err != nil {
		return fmt.Errorf(
			"unable to access system keychain: %w",
			err,
		)
	}

	return nil
}

func Load() (string, string, error) {
	email, err := keyring.Get(Service, EmailKey)
	if err != nil {
		return "", "", err
	}

	password, err := keyring.Get(Service, TokenKey)
	if err != nil {
		return "", "", err
	}

	return email, password, nil
}

func Exists() bool {
	_, err := keyring.Get(Service, EmailKey)
	return err == nil
}

func Delete() error {
	if err := keyring.Delete(Service, EmailKey); err != nil {
		return err
	}

	if err := keyring.Delete(Service, TokenKey); err != nil {
		return err
	}

	return nil
}
