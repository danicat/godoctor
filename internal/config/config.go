package config

import (
	"flag"
)

type Config struct {
	ListenAddr   string
	Version      bool
	DefaultModel string
}

func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("godoctor", flag.ContinueOnError)
	versionFlag := fs.Bool("version", false, "print the version and exit")
	listenAddr := fs.String("listen", "", "listen address for HTTP transport (e.g., :8080)")
	defaultModel := fs.String("model", "gemini-2.5-pro", "default Gemini model to use")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := &Config{
		ListenAddr:   *listenAddr,
		Version:      *versionFlag,
		DefaultModel: *defaultModel,
	}

	return cfg, nil
}
