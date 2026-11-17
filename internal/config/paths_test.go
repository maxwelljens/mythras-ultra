// SPDX-License-Identifier: EUPL-1.2

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maxwelljens/mythras-ultra/internal/config"
)

func TestConfigDir(t *testing.T) {
	home := t.TempDir()

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "MYUL_CONFIG_DIR wins",
			env: map[string]string{
				"MYUL_CONFIG_DIR": "/custom/myul",
				"XDG_CONFIG_HOME": "/xdg",
				"HOME":            home,
			},
			want: "/custom/myul",
		},
		{
			name: "XDG_CONFIG_HOME",
			env: map[string]string{
				"MYUL_CONFIG_DIR": "",
				"XDG_CONFIG_HOME": "/xdg",
				"HOME":            home,
			},
			want: filepath.Join("/xdg", "myul"),
		},
		{
			name: "home fallback",
			env: map[string]string{
				"MYUL_CONFIG_DIR": "",
				"XDG_CONFIG_HOME": "",
				"HOME":            home,
			},
			want: filepath.Join(home, ".config", "myul"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			if got := config.ConfigDir(); got != tt.want {
				t.Errorf("ConfigDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDataDir(t *testing.T) {
	home := t.TempDir()

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "MYUL_DATA_DIR wins",
			env: map[string]string{
				"MYUL_DATA_DIR": "/custom/data",
				"XDG_DATA_HOME": "/xdg",
				"HOME":          home,
			},
			want: "/custom/data",
		},
		{
			name: "XDG_DATA_HOME",
			env: map[string]string{
				"MYUL_DATA_DIR": "",
				"XDG_DATA_HOME": "/xdg",
				"HOME":          home,
			},
			want: filepath.Join("/xdg", "myul"),
		},
		{
			name: "home fallback",
			env: map[string]string{
				"MYUL_DATA_DIR": "",
				"XDG_DATA_HOME": "",
				"HOME":          home,
			},
			want: filepath.Join(home, ".local", "share", "myul"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			if got := config.DataDir(); got != tt.want {
				t.Errorf("DataDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionsDir(t *testing.T) {
	data := t.TempDir()
	t.Setenv("MYUL_DATA_DIR", data)

	want := filepath.Join(data, "sessions")
	if got := config.SessionsDir(); got != want {
		t.Errorf("SessionsDir() = %q, want %q", got, want)
	}
}

func TestResolveConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MYUL_CONFIG_DIR", dir)

	defaultPath := filepath.Join(dir, "config.toml")
	if got := config.DefaultConfigPath(); got != defaultPath {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, defaultPath)
	}

	tests := []struct {
		name     string
		explicit string
		want     string
	}{
		{name: "empty uses default", explicit: "", want: defaultPath},
		{name: "explicit wins", explicit: "/etc/myul.toml", want: "/etc/myul.toml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.ResolveConfigPath(tt.explicit); got != tt.want {
				t.Errorf("ResolveConfigPath(%q) = %q, want %q", tt.explicit, got, tt.want)
			}
		})
	}
}

func TestEnsureDirs(t *testing.T) {
	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("MYUL_CONFIG_DIR", cfgDir)
	t.Setenv("MYUL_DATA_DIR", dataDir)

	if err := config.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	// Second call must be idempotent.
	if err := config.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() second call error = %v", err)
	}

	for _, dir := range []string{config.ConfigDir(), config.DataDir(), config.SessionsDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("stat %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
		// Owner rwx must be granted; group/other bits are subject to
		// the process umask, so they are not asserted.
		if perm := info.Mode().Perm(); perm&0o700 != 0o700 {
			t.Errorf("%s permissions = %o, want owner rwx", dir, perm)
		}
	}
}
