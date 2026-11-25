package main

import (
	"errors"
	"strings"
)

const minJWTSecretBytes = 32

func validateJWTSecret(secret string) error {
	if len(strings.Join(strings.Fields(secret), "")) < minJWTSecretBytes {
		return errors.New("JWT_SECRET must contain at least 32 non-whitespace bytes")
	}
	return nil
}

func serverUsesTLS(appEnv, certFile, keyFile string) (bool, error) {
	appEnv = strings.ToLower(strings.TrimSpace(appEnv))
	certFile = strings.TrimSpace(certFile)
	keyFile = strings.TrimSpace(keyFile)

	if appEnv != "development" && appEnv != "production" {
		return false, errors.New("APP_ENV must be set to development or production")
	}
	if certFile == "" && keyFile == "" && appEnv == "development" {
		return false, nil
	}
	if certFile == "" || keyFile == "" {
		return false, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must both be set")
	}
	return true, nil
}

func serverAddress(useTLS bool) string {
	if useTLS {
		return ":8080"
	}
	return "127.0.0.1:8080"
}
