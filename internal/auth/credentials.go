package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Credentials struct {
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
}

var errCredentials = errors.New("Codex ChatGPT credentials unavailable. Sign in through Codex.")

// Load reads the current snapshot without modifying Codex's credentials or renewal state.
func Load() (Credentials, error) {
	directory := os.Getenv("CODEX_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Credentials{}, errCredentials
		}
		directory = filepath.Join(home, ".codex")
	}
	return loadFile(filepath.Join(directory, "auth.json"))
}

func loadFile(path string) (Credentials, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, errCredentials
	}
	var snapshot struct {
		Tokens Credentials `json:"tokens"`
	}
	if json.Unmarshal(contents, &snapshot) != nil || snapshot.Tokens.AccessToken == "" || snapshot.Tokens.AccountID == "" {
		return Credentials{}, errCredentials
	}
	return snapshot.Tokens, nil
}
