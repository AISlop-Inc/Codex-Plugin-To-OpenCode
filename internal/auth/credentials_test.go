package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRereadsCodexHome(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("CODEX_HOME", directory)
	path := filepath.Join(directory, "auth.json")
	for _, token := range []string{"first", "refreshed"} {
		contents := `{"tokens":{"access_token":"` + token + `","account_id":"account-a","refresh_token":"unused"}}`
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		credentials, err := Load()
		if err != nil || credentials.AccessToken != token || credentials.AccountID != "account-a" {
			t.Fatal("credentials were not reloaded")
		}
	}
}

func TestInvalidCredentialsDoNotLeakContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	for _, contents := range []string{
		`{"OPENAI_API_KEY":"private-value"}`,
		`{"tokens":{"access_token":"private-value","account_id":123}}`,
		`{"tokens":{"access_token":"private-value"}}`,
		`invalid-private-value`,
	} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := loadFile(path)
		if err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatal("invalid credentials must fail without disclosing contents")
		}
	}
	if _, err := loadFile(path + "-missing"); err == nil {
		t.Fatal("missing credentials must fail")
	}
}
