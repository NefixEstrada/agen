// Package config loads the agen generation configuration (agen.yml).
package config

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"
)

// Config is the root of the agen configuration file.
type Config struct {
	// Target configures the generated package.
	Target Target `json:"target" yaml:"target"`
	// Parser configures spec parsing.
	Parser Parser `json:"parser" yaml:"parser"`
	// Generator configures code generation.
	Generator Generator `json:"generator" yaml:"generator"`
	// Broker configures runtime backend defaults.
	Broker Broker `json:"broker" yaml:"broker"`
	// Expand optionally dumps the fully-dereferenced spec.
	Expand Expand `json:"expand" yaml:"expand"`
}

// Target configures the generated package.
type Target struct {
	// PackageName of the generated package. Default: derived from info.title.
	PackageName string `json:"package_name" yaml:"package_name"`
	// Dir to write generated code to. Default: ./api.
	Dir string `json:"dir" yaml:"dir"`
	// Spec URL or path to the AsyncAPI document. Required.
	Spec string `json:"spec" yaml:"spec"`
}

// Parser configures spec parsing.
type Parser struct {
	// InferTypes enables schema type inference when `type` is not set.
	InferTypes bool `json:"infer_types" yaml:"infer_types"`
	// AllowRemote enables remote (http/https) references.
	AllowRemote bool `json:"allow_remote" yaml:"allow_remote"`
	// DepthLimit limits reference resolution depth. Default 1000.
	DepthLimit int `json:"depth_limit" yaml:"depth_limit"`
	// IgnoreUnsupported lists unsupported features to skip instead of erroring,
	// e.g. ["schema.not", "operation.reply"] or ["all"].
	IgnoreUnsupported []string `json:"ignore_unsupported" yaml:"ignore_unsupported"`
}

// Generator configures code generation.
type Generator struct {
	// Features enable/disable generation features.
	Features Features `json:"features" yaml:"features"`
	// Filters select a subset of operations.
	Filters Filters `json:"filters" yaml:"filters"`
	// Initialisms applies the initialism naming rules (e.g. "Id" -> "ID")
	// to generated identifiers.
	Initialisms bool `json:"initialisms" yaml:"initialisms"`
}

// Features is the feature enable/disable configuration.
type Features struct {
	// Enable lists features to enable.
	Enable []Feature `json:"enable" yaml:"enable"`
	// Disable lists features to disable.
	Disable []Feature `json:"disable" yaml:"disable"`
	// DisableAll disables every feature.
	DisableAll bool `json:"disable_all" yaml:"disable_all"`
}

// Feature is a codegen feature name in configuration.
type Feature = string

// Filters select a subset of operations to generate.
type Filters struct {
	// OperationsRegex keeps only matching operations.
	OperationsRegex string `json:"operations_regex" yaml:"operations_regex"`
	// Actions keeps only the given actions: "send" and/or "receive".
	Actions []string `json:"actions" yaml:"actions"`
}

// Broker configures runtime backends.
type Broker struct {
	// Redis configures the Redis backend defaults.
	Redis *Redis `json:"redis" yaml:"redis"`
}

// Redis configures the Redis backend.
type Redis struct {
	// Module is the go-redis module path the generated code is used with.
	Module string `json:"module" yaml:"module"`
	// Mode is "streams" (default) or "pubsub".
	Mode string `json:"mode" yaml:"mode"`
}

// Expand optionally dumps the fully-dereferenced spec.
type Expand struct {
	// Output path for the expanded spec dump; empty disables it.
	Output string `json:"output" yaml:"output"`
}

// configFiles are auto-discovered configuration file names.
var configFiles = []string{
	"agen.yml", "agen.yaml",
	".agen.yml", ".agen.yaml",
}

// AutoDiscover looks for a configuration file in dir and its parents.
func AutoDiscover(dir string) (path string, found bool) {
	for {
		for _, f := range configFiles {
			p := filepath.Join(dir, f)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// Load loads the configuration from the given file, strictly: unknown fields
// are errors.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read config")
	}
	var cfg Config
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(&cfg); err != nil {
		return nil, errors.Wrap(err, "decode config (strict: unknown fields are errors)")
	}
	if err := cfg.setDefaults(); err != nil {
		return nil, err
	}
	// Relative paths in the config resolve against the config file directory.
	if !filepath.IsAbs(cfg.Target.Spec) {
		if rel, err := filepath.Abs(filepath.Join(filepath.Dir(path), cfg.Target.Spec)); err == nil {
			cfg.Target.Spec = rel
		}
	}
	if !filepath.IsAbs(cfg.Target.Dir) {
		if rel, err := filepath.Abs(filepath.Join(filepath.Dir(path), cfg.Target.Dir)); err == nil {
			cfg.Target.Dir = rel
		}
	}
	return &cfg, nil
}

// SetDefaults validates and fills unset fields.
func (c *Config) SetDefaults() error { return c.setDefaults() }

func (c *Config) setDefaults() error {
	if c.Target.Dir == "" {
		c.Target.Dir = "./api"
	}
	switch c.Broker.RedisMode() {
	case "", "streams", "pubsub":
	default:
		return errors.Errorf("broker.redis.mode: invalid value %q (streams or pubsub)", c.Broker.RedisMode())
	}
	for _, a := range c.Generator.Filters.Actions {
		if a != "send" && a != "receive" {
			return errors.Errorf("generator.filters.actions: invalid action %q (send or receive)", a)
		}
	}
	return nil
}

// RedisMode returns the configured Redis mode, defaulting to streams.
func (b Broker) RedisMode() string {
	if b.Redis == nil || b.Redis.Mode == "" {
		return "streams"
	}
	return b.Redis.Mode
}
