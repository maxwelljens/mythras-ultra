// SPDX-License-Identifier: EUPL-1.2

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// configFileName is the config file inside [ConfigDir].
const configFileName = "config.toml"

// ConfigDir returns the directory holding myul's configuration.
//
// Order: $MYUL_CONFIG_DIR, then $XDG_CONFIG_HOME/myul, then
// ~/.config/myul. If the home directory cannot be resolved the result
// is relative to the working directory (.config/myul).
func ConfigDir() string {
	if dir := os.Getenv("MYUL_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "myul")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "myul")
	}
	return filepath.Join(home, ".config", "myul")
}

// DataDir returns the directory holding myul's session data.
//
// Order: $MYUL_DATA_DIR, then $XDG_DATA_HOME/myul, then
// ~/.local/share/myul. If the home directory cannot be resolved the
// result is relative to the working directory (.local/share/myul).
func DataDir() string {
	if dir := os.Getenv("MYUL_DATA_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "myul")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "myul")
	}
	return filepath.Join(home, ".local", "share", "myul")
}

// SessionsDir returns the directory holding session state
// (<data dir>/sessions, ARCHITECTURE §5).
func SessionsDir() string {
	return filepath.Join(DataDir(), "sessions")
}

// DefaultConfigPath returns the config file [Load] reads when no
// explicit --config path is given: <config dir>/config.toml.
func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), configFileName)
}

// ResolveConfigPath returns explicit when it is non-empty (the --config
// flag), otherwise [DefaultConfigPath].
func ResolveConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return DefaultConfigPath()
}

// EnsureDirs creates [ConfigDir], [DataDir] and [SessionsDir] (and any
// missing parents) with permissions 0o755 if they do not exist yet.
// It is idempotent.
func EnsureDirs() error {
	for _, dir := range []string{ConfigDir(), DataDir(), SessionsDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: create directory %s: %w", dir, err)
		}
	}
	return nil
}
