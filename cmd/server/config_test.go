package main

import (
	"strings"
	"testing"
)

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "empty", secret: "", wantErr: true},
		{name: "short", secret: strings.Repeat("x", minJWTSecretBytes-1), wantErr: true},
		{name: "whitespace", secret: strings.Repeat(" ", minJWTSecretBytes), wantErr: true},
		{name: "embedded whitespace does not count", secret: strings.Repeat("x ", minJWTSecretBytes/2-1), wantErr: true},
		{name: "minimum length", secret: strings.Repeat("x", minJWTSecretBytes)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateJWTSecret(test.secret); (err != nil) != test.wantErr {
				t.Fatalf("validateJWTSecret() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestServerUsesTLS(t *testing.T) {
	tests := []struct {
		name     string
		appEnv   string
		certFile string
		keyFile  string
		wantTLS  bool
		wantErr  bool
	}{
		{name: "development permits local HTTP", appEnv: "development"},
		{name: "production requires TLS", appEnv: "production", wantErr: true},
		{name: "partial TLS configuration rejected", appEnv: "development", certFile: "server.crt", wantErr: true},
		{name: "production enables TLS", appEnv: "production", certFile: "server.crt", keyFile: "server.key", wantTLS: true},
		{name: "unknown environment rejected", appEnv: "staging", wantErr: true},
		{name: "missing environment rejected", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useTLS, err := serverUsesTLS(test.appEnv, test.certFile, test.keyFile)
			if (err != nil) != test.wantErr {
				t.Fatalf("serverUsesTLS() error = %v, wantErr %v", err, test.wantErr)
			}
			if useTLS != test.wantTLS {
				t.Fatalf("serverUsesTLS() = %v, want %v", useTLS, test.wantTLS)
			}
		})
	}
}

func TestServerAddress(t *testing.T) {
	tests := []struct {
		name   string
		useTLS bool
		want   string
	}{
		{name: "development binds to loopback", want: "127.0.0.1:8080"},
		{name: "TLS server accepts remote connections", useTLS: true, want: ":8080"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := serverAddress(test.useTLS); got != test.want {
				t.Fatalf("serverAddress(%v) = %q, want %q", test.useTLS, got, test.want)
			}
		})
	}
}
