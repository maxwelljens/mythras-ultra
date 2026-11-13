// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mythras-ultra/myul/internal/config"
)

// execute runs the root command with args and returns everything it
// printed on stdout/stderr.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// blankEnv clears the MYUL_* keys doctor and config read, so the
// ambient environment cannot change test outcomes (Viper treats an
// empty variable as unset).
func blankEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"MYUL_DECISION_ENABLED", "MYUL_DECISION_URL", "MYUL_DECISION_MODEL",
		"MYUL_GAME_SEED", "MYUL_UI_COLOUR_MODE", "MYUL_UI_THEME",
	} {
		t.Setenv(key, "")
	}
}

func TestVersionCmd(t *testing.T) {
	out, err := execute(t, "version")
	if err != nil {
		t.Fatalf("version error = %v", err)
	}
	want := "myul " + version + " (" + buildDate + ")\n"
	if out != want {
		t.Errorf("version output = %q, want %q", out, want)
	}
}

func TestConfigPathCmd(t *testing.T) {
	blankEnv(t)
	dir := t.TempDir()
	t.Setenv("MYUL_CONFIG_DIR", dir)

	out, err := execute(t, "config", "path")
	if err != nil {
		t.Fatalf("config path error = %v", err)
	}
	want := filepath.Join(dir, "config.toml") + "\n"
	if out != want {
		t.Errorf("config path output = %q, want %q", out, want)
	}

	out, err = execute(t, "--config", "/custom/myul.toml", "config", "path")
	if err != nil {
		t.Fatalf("config path --config error = %v", err)
	}
	if want := "/custom/myul.toml\n"; out != want {
		t.Errorf("config path --config output = %q, want %q", out, want)
	}
}

func TestConfigInit(t *testing.T) {
	blankEnv(t)
	dir := t.TempDir()
	t.Setenv("MYUL_CONFIG_DIR", dir)
	path := filepath.Join(dir, "config.toml")

	if _, err := execute(t, "config", "init"); err != nil {
		t.Fatalf("config init error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("config path is not a regular file: %v", info.Mode())
	}

	// The written template must load as exactly the default config —
	// this keeps defaultConfigTOML and config.DefaultConfig in sync.
	got, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() written config error = %v", err)
	}
	if want := config.DefaultConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("Load() written config = %+v, want %+v", got, want)
	}

	// A second init must refuse to overwrite...
	if _, err := execute(t, "config", "init"); err == nil {
		t.Error("config init on existing file: want error, got nil")
	}
	// ...unless --force is given.
	if _, err := execute(t, "config", "init", "--force"); err != nil {
		t.Fatalf("config init --force error = %v", err)
	}
}

func TestDoctorInvalidConfig(t *testing.T) {
	blankEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[ui]\ncolour_mode = \"neon\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := execute(t, "--config", path, "doctor"); err == nil {
		t.Error("doctor with invalid config: want error, got nil")
	}
}

func TestDoctorReachable(t *testing.T) {
	blankEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("probe path = %q, want /api/tags", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "config.toml")
	contents := "[decision]\nurl = " + strconv.Quote(srv.URL) + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, err := execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v (reachability must not fail the command)", err)
	}
	if !strings.Contains(out, "ok: config valid") {
		t.Errorf("doctor output missing config line: %q", out)
	}
	if !strings.Contains(out, "ok: decision model reachable") {
		t.Errorf("doctor output missing reachability line: %q", out)
	}
}

func TestDoctorUnreachable(t *testing.T) {
	blankEnv(t)
	// Port 1 on loopback refuses connections instantly; nothing can
	// listen on it without root.
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := "[decision]\nurl = \"http://127.0.0.1:1\"\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, err := execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v (unreachable model must not fail the command)", err)
	}
	if !strings.Contains(out, "failed: decision model unreachable") {
		t.Errorf("doctor output missing failure line: %q", out)
	}
}
