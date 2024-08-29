package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	_ "embed"

	"gopkg.in/yaml.v3"
)

var (
	//go:embed defaults.yaml
	defaults      []byte
	defaultConfig Config
)

func init() {
	err := yaml.Unmarshal(defaults, &defaultConfig)
	if err != nil {
		panic(fmt.Errorf("error parsing defaults: %w", err))
	}
}

type (
	// ServerConfig holds the configuration for the server, including the address and port.
	ServerConfig struct {
		Address  string `yaml:"address" json:"address"`
		Port     uint16 `yaml:"port" json:"port"`
		DataPath string `yaml:"dataPath" json:"dataPath"`
	}

	// FilterConfig holds the regular expression patterns used to filter destinations.
	FilterConfig struct {
		To             []*regexp.Regexp `yaml:"to" json:"to"`
		MaxMessageSize uint             `yaml:"maxMessageSize" json:"maxMessageSize"`
	}

	// ProxyConfig holds the configuration for the proxy server.
	ProxyConfig struct {
		Address  string   `yaml:"address" json:"address"`
		Port     uint16   `yaml:"port" json:"port"`
		Username string   `yaml:"username" json:"username"`
		Password Password `yaml:"password" json:"password"`
		// TODO: override some fields (eg `From` address)
		// this requires parsing the email body (it's just mime/multipart)
		// and replacing the `From` field with a properly formatted one.
	}

	// Password holds passwords
	// TODO: bcrypt/scrypt/argon2id encrypted? accept all?
	Password string

	// AuthConfig holds the configuration for the authentication server.
	AuthConfig struct {
		Plain map[string]Password `yaml:"plain" json:"plain"`
	}

	// Config holds the configuration for the application.
	Config struct {
		Server   ServerConfig `yaml:"server" json:"server"`
		Filters  FilterConfig `yaml:"filters" json:"filter"`
		Proxy    ProxyConfig  `yaml:"proxy" json:"proxy"`
		Auth     AuthConfig   `yaml:"auth" json:"auth"`
		LogLevel slog.Level   `yaml:"logLevel" json:"logLevel"`
	}
)

// Clone the config
func (c *Config) Clone() *Config {
	return &Config{
		Server: ServerConfig{
			Address:  c.Server.Address,
			Port:     c.Server.Port,
			DataPath: c.Server.DataPath,
		},
		Filters: FilterConfig{
			To:             slices.Clone(c.Filters.To),
			MaxMessageSize: c.Filters.MaxMessageSize,
		},
		Proxy: c.Proxy,
		Auth: AuthConfig{
			Plain: maps.Clone(c.Auth.Plain),
		},
		LogLevel: c.LogLevel,
	}
}

// String returns the address and port of the server configuration as a string.
func (sc ServerConfig) String() string {
	return string(strconv.AppendUint([]byte(sc.Address+":"), uint64(sc.Port), 10))
}

// String returns a string representation of the Password, hiding the actual password value.
func (p Password) String() string {
	return "**redacted**"
}

func (p Password) MarshalJSON() ([]byte, error) {
	return []byte(`"**redacted**"`), nil
}

// Addr returns the address and port of the proxy configuration as a string.
func (pc ProxyConfig) Addr() string {
	if pc.Port == 0 {
		return pc.Address
	}
	return string(strconv.AppendUint([]byte(pc.Address+":"), uint64(pc.Port), 10))
}

// parseConfigFile reads a configuration file
func parseConfigFile(reader io.Reader, previous *Config) (*Config, error) {
	config := previous.Clone()
	err := yaml.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %w", err)
	}

	return config, nil
}

type ConfigStore struct {
	state  atomic.Pointer[Config]
	target string
}

// NewConfigStore creates a new ConfigStore instance and initializes it with the given configuration path.
func NewConfigStore(configPath string) (*ConfigStore, error) {
	cs := &ConfigStore{
		state:  atomic.Pointer[Config]{},
		target: configPath,
	}
	cs.Background(5 * time.Second)
	return cs, cs.reload(time.Time{})
}

// Load returns the current configuration state.
func (cs *ConfigStore) Load() Config {
	return *cs.state.Load()
}

func (cs *ConfigStore) reload(lastSync time.Time) error {
	fileInfo, err := os.Stat(cs.target)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	if fileInfo.ModTime().Before(lastSync) {
		return nil
	}

	f, err := os.Open(cs.target)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	c, err := parseConfigFile(f, &defaultConfig)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	buf, _ := json.Marshal(c)
	cs.state.Store(c)

	slog.SetLogLoggerLevel(c.LogLevel)
	slog.Debug("loaded new config", "config", string(buf))

	return nil
}

// Background starts a goroutine that periodically reloads the configuration.
func (cs *ConfigStore) Background(dur time.Duration) {
	ticker := time.NewTicker(dur)
	go func() {
		for t := range ticker.C {
			err := cs.reload(t.Add(-dur))
			if err != nil {
				slog.Error("failed to reload config", "err", err)
			}
		}
	}()
}

// ConfigLoader defines an interface for loading configurations.
type ConfigLoader interface {
	Load() Config
}
