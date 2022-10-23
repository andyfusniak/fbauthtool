package config

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	configDir  = ".fbauthtool"
	configFile = "config.json"
)

// Config application configuration.
type Config struct {
	Current *Item
	Cfgs    map[string]Item
}

// Item single configuration.
type Item struct {
	FBCreds []byte
}

// serviceAccountsConfig container for entire configuration file.
type serviceAccountsConfig struct {
	ServiceAccounts []serviceAccountRef `json:"serviceAccounts"`
}

// serviceAccountRef service account key reference container.
type serviceAccountRef struct {
	ProjectID string `json:"projectId"`
	Filepath  string `json:"filepath"`
}

// NewConfigFromFile creates the config from the configuration JSON file.
func NewConfigFromFile() (*Config, error) {
	hd, err := homeDir()
	if err != nil {
		return nil, errors.Wrapf(err, "[run] failed to get home dir")
	}

	filename := filepath.Join(hd, configDir, configFile)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	sac := serviceAccountsConfig{}
	if err := dec.Decode(&sac); err != nil {
		return nil, err
	}

	// fmt.Printf("%#v\n", sac)

	cfg := Config{
		Cfgs: make(map[string]Item),
	}
	for i, v := range sac.ServiceAccounts {
		b, err := os.ReadFile(v.Filepath)
		if err != nil {
			return nil, err
		}
		item := Item{
			FBCreds: b,
		}
		cfg.Cfgs[v.ProjectID] = item
		if i == 0 {
			cfg.Current = &item
		}
	}
	return &cfg, nil
}

func homeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", errors.Wrap(err, "user.Current()")
	}
	return usr.HomeDir, nil
}
