// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main is the entry point for the godoctor CLI and MCP server.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/danicat/godoctor/internal/cli"
)

const defaultVersion = "dev"

var (
	version = defaultVersion
)

func init() {
	if version == defaultVersion || version == "" {
		version = resolveVersionFromBuildInfo(version)
	}
}

func resolveVersionFromBuildInfo(fallback string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		if fallback != "" {
			return fallback
		}
		return defaultVersion
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" && info.Main.Version != defaultVersion {
		return info.Main.Version
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			if setting.Value == "true" {
				modified = true
			}
		}
	}

	if revision != "" {
		if len(revision) > 7 {
			revision = revision[:7]
		}
		if modified {
			return fmt.Sprintf("devel (%s-dirty)", revision)
		}
		return fmt.Sprintf("devel (%s)", revision)
	}

	if fallback != "" {
		return fallback
	}
	return defaultVersion
}

func main() {
	os.Exit(runMain())
}

func runMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err := run(ctx, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string) error {
	return cli.Run(ctx, version, args, os.Stdin, os.Stdout, os.Stderr)
}
