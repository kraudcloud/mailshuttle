package main

import (
	"fmt"
	"io"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds the configuration for the server, including the address and port.
type ServerConfig struct {
	Address  string `yaml:"address"`
	Port     uint16 `yaml:"port"`
	DataPath string `yaml:"dataPath"`
}

// String returns the address and port of the server configuration as a string.
func (sc ServerConfig) String() string {
	return string(strconv.AppendUint([]byte(sc.Address+":"), uint64(sc.Port), 10))
}

// FilterConfig holds the regular expression patterns used to filter destinations.
type FilterConfig struct {
	To             []*regexp.Regexp
	MaxMessageSize uint `yaml:"maxMessageSize"`
}

// ProxyConfig holds the configuration for the proxy server.
type ProxyConfig struct {
	Address  string `yaml:"address"`
	Port     uint16 `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// TODO: override some fields (eg `From` address)
	// this requires parsing the email body (it's just mime/multipart)
	// and replacing the `From` field with a properly formatted one.
}

// Addr returns the address and port of the proxy configuration as a string.
func (pc ProxyConfig) Addr() string {
	if pc.Port == 0 {
		return pc.Address
	}
	return string(strconv.AppendUint([]byte(pc.Address+":"), uint64(pc.Port), 10))
}

// AuthConfig holds the configuration for the authentication server.
type AuthConfig struct {
	Plain map[string]Password `yaml:"plain"`
}

// Config holds the configuration for the application.
type Config struct {
	Server   ServerConfig
	Filter   FilterConfig
	Proxy    ProxyConfig
	Auth     AuthConfig
	LogLevel int `yaml:"log_level"`
}

type ConfigFile struct {
	Server  ServerConfig `yaml:"server"`
	Filters struct {
		DestinationPatterns []string `yaml:"to"`
		MaxMessageSize      uint     `yaml:"maxMessageSize"`
	} `yaml:"filters"`
	Proxy    ProxyConfig `yaml:"proxy"`
	Auth     AuthConfig  `yaml:"auth"`
	LogLevel int         `yaml:"logLevel"`
}

// ParseConfig reads a configuration file and returns a Config struct.
// The configuration file is expected to be in YAML format and contain the following fields:
//
//		server:
//	  	  address: <address>
//	  	  port: <port>
//		filters:
//	  	  to:
//		    - <destination_pattern_1>
//	        - <destination_pattern_2>
//	        - ...
//		proxy:
//	  	  address: <proxy_address>
//	  	  port: <proxy_port>
//	  	  username: <proxy_username>
//	  	  password: <proxy_password>
//		auth:
//		  plain:
//		    hitori: gotou
func ParseConfig(reader io.Reader) (*Config, error) {
	config := ConfigFile{
		Server: ServerConfig{
			Address:  "0.0.0.0",
			Port:     2525,
			DataPath: "/var/lib/mailshuttle/mails",
		},
		Filters: struct {
			DestinationPatterns []string "yaml:\"to\""
			MaxMessageSize      uint     "yaml:\"maxMessageSize\""
		}{
			MaxMessageSize: 1024 * 1024 * 8,
		},
		Proxy: ProxyConfig{
			Port: 587,
		},
	}
	err := yaml.NewDecoder(reader).Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %w", err)
	}

	c := Config{
		Server: config.Server,
		Filter: FilterConfig{
			To:             make([]*regexp.Regexp, len(config.Filters.DestinationPatterns)),
			MaxMessageSize: config.Filters.MaxMessageSize,
		},
		Proxy:    config.Proxy,
		Auth:     config.Auth,
		LogLevel: config.LogLevel,
	}

	for i, pattern := range config.Filters.DestinationPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("error compiling regexp: %w", err)
		}
		c.Filter.To[i] = re
	}

	return &c, nil
}

func DumpConfig(writer io.Writer, c *Config) error {
	config := ConfigFile{
		Server: c.Server,
		Filters: struct {
			DestinationPatterns []string "yaml:\"to\""
			MaxMessageSize      uint     "yaml:\"maxMessageSize\""
		}{
			MaxMessageSize:      c.Filter.MaxMessageSize,
			DestinationPatterns: make([]string, len(c.Filter.To)),
		},
		Proxy:    c.Proxy,
		Auth:     c.Auth,
		LogLevel: c.LogLevel,
	}

	for i, pattern := range c.Filter.To {
		config.Filters.DestinationPatterns[i] = pattern.String()
	}

	return yaml.NewEncoder(writer).Encode(config)
}
