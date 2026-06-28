// Package config loads the application's settings from a YAML file and/or
// environment variables into a typed Config struct.
//
// Why config lives outside the code: settings that change between environments
// (dev/staging/prod) — ports, DB paths — should NOT be hardcoded or require a
// recompile. This follows the 12-Factor App "Config" principle.
package config

import (
	"flag"
	"log" // used for Fatal logging (logs then calls os.Exit)
	"os"  // used to talk to the operating system (env vars, files)

	"github.com/ilyakaznacheev/cleanenv"
)

// HTTPServer groups the HTTP server settings. The cleanenv library reads three
// tags per field to build a precedence chain (env var > YAML file > default):
//
//	yaml:"address"               -> key to read from the YAML file
//	env:"HTTP_ADDRESS"           -> env var that can OVERRIDE the file value
//	env-default:"localhost:8080" -> fallback if neither is set
type HTTPServer struct {
	Addr string `yaml:"address" env:"HTTP_ADDRESS" env-default:"localhost:8080"`
}

// Config is the root settings struct.
//
// HTTPServer is EMBEDDED (note: no field name). Go has no inheritance; it uses
// composition via embedding. The embedded struct's fields are "promoted", so
// callers can write cfg.Addr directly instead of cfg.HTTPServer.Addr.
//
// env-required:"true" on StoragePath makes loading FAIL if it's set nowhere —
// fail-fast on missing critical config.
type Config struct {
	Env         string `yaml:"env" env:"ENV" env-default:"production"`
	StoragePath string `yaml:"storage_path" env-default:"storage/storage.db" env-required:"true"`
	HTTPServer  `yaml:"http_server"`
}

// MustLoad loads the config or terminates the program.
//
// The "Must" prefix is a Go convention (see regexp.MustCompile, template.Must)
// meaning "panic/exit on failure instead of returning an error". It's the right
// choice for config: an app with no valid config can't run, so crashing at
// startup — before serving any traffic — is correct.
//
// Note: log.Fatal calls os.Exit, which means DEFERRED functions do NOT run.
// That's fine here (nothing to clean up yet), but avoid log.Fatal in code that
// holds resources that need releasing.
func MustLoad() *Config {
	var configPath string

	// 1) Prefer the CONFIG_PATH environment variable. Getenv returns "" if unset.
	configPath = os.Getenv("CONFIG_PATH")

	if configPath == "" {
		// 2) Otherwise fall back to a command-line flag: -config <path>.
		// flag.String returns a *string (pointer); the value isn't populated
		// until flag.Parse() runs, which is why we dereference with *flags after.
		flags := flag.String("config", "", "path to configuration file")
		flag.Parse()

		configPath = *flags

		// 3) Neither source provided a path -> we can't continue.
		if configPath == "" {
			log.Fatal("Config path is not set")
		}
	}

	// 4) Verify the file exists, to give a clear error instead of a cryptic
	// parse failure later.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist: %s", configPath)
	}

	var cfg Config

	// 5) Parse the file (and apply env overrides/defaults) INTO cfg. We pass
	// &cfg (a pointer) so the library can fill our struct.
	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		log.Fatalf("Can not read config file: %s", err.Error())
	}

	return &cfg
}
