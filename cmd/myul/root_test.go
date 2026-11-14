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
	"time"

	"github.com/mythras-ultra/myul/internal/config"
)

// execute runs the root command with args and returns everything it
// printed on stdout/stderr. The args slice is always non-nil: cobra
// falls back to os.Args[1:] for a nil slice, which would hand the test
// binary's own flags to the command.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append([]string{}, args...))
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
		"MYUL_LLM_PROVIDER", "MYUL_LLM_MODEL", "MYUL_LLM_API_KEY_ENV", "MYUL_LLM_BASE_URL",
		"MYUL_LLM_HEADERS",
	} {
		t.Setenv(key, "")
	}
}

// isolatedDirs points the config and data directories at fresh temp
// dirs, so a test's commands never read the developer's configuration or
// write into the real session store.
func isolatedDirs(t *testing.T) (dataDir, configDir string) {
	t.Helper()
	blankEnv(t)
	dataDir = t.TempDir()
	configDir = t.TempDir()
	t.Setenv("MYUL_DATA_DIR", dataDir)
	t.Setenv("MYUL_CONFIG_DIR", configDir)
	return dataDir, configDir
}

// writeConfig writes a config file at path (0600, like the config
// package's own tests).
func writeConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config %s: %v", path, err)
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

// decisionConfig writes a config file whose [decision] section points at
// url with the given model tag.
func decisionConfig(t *testing.T, url, model string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := "[decision]\nurl = " + strconv.Quote(url) + "\nmodel = " + strconv.Quote(model) + "\n"
	writeConfig(t, path, contents)
	return path
}

// TestDoctorDecisionModelAvailable drives the real decide client's
// Healthy probe: a server that lists the configured model reports ok.
func TestDoctorDecisionModelAvailable(t *testing.T) {
	blankEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("probe path = %q, want /api/tags", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"tev1:4b"}]}`))
	}))
	defer srv.Close()

	path := decisionConfig(t, srv.URL, "tev1:4b")
	out, err := execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v", err)
	}
	if !strings.Contains(out, "ok: decision model available (tev1:4b)") {
		t.Errorf("doctor output missing model check line: %q", out)
	}
}

// TestDoctorDecisionModelMissing: the endpoint answers but does not serve
// the configured model. Like reachability, that is a degraded state
// (ARCHITECTURE §3.3) — reported as failed, exit code stays clean.
func TestDoctorDecisionModelMissing(t *testing.T) {
	blankEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:8b"}]}`))
	}))
	defer srv.Close()

	path := decisionConfig(t, srv.URL, "tev1:4b")
	out, err := execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v (a missing model must not fail the command)", err)
	}
	if !strings.Contains(out, "ok: decision model reachable") {
		t.Errorf("doctor output missing reachability line: %q", out)
	}
	if !strings.Contains(out, "failed: decision model check") {
		t.Errorf("doctor output missing model check failure: %q", out)
	}
}

// TestDoctorAPIKeyEnvProbe: doctor reports whether the configured
// provider's effective api-key env variable is set — ok / failed: $XX
// unset — resolving the variable from the catalog when the config leaves
// api_key_env empty. Like the other probes it never fails the exit code.
// The endpoint probe needs a local base_url so the test stays offline.
func TestDoctorAPIKeyEnvProbe(t *testing.T) {
	blankEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	writeConfig(t, path,
		"[llm]\nprovider = \"deepseek\"\nmodel = \"deepseek-chat\"\nbase_url = \"http://127.0.0.1:1\"\n")
	// Forced unset so the ambient environment cannot satisfy the probe.
	t.Setenv("DEEPSEEK_API_KEY", "")

	out, err := execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor with an unset API key must not fail: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "failed: $DEEPSEEK_API_KEY unset") {
		t.Errorf("doctor output missing the unset-key line: %q", out)
	}

	t.Setenv("DEEPSEEK_API_KEY", "sk-test-value")
	out, err = execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v (output %q)", err, out)
	}
	if !strings.Contains(out, "ok: $DEEPSEEK_API_KEY set") {
		t.Errorf("doctor output missing the set-key line: %q", out)
	}
}

// TestDoctorLLMEndpoint probes <base>/models against a local server: a
// 200 reports ok with the configured key and headers attached; a 401
// surfaces the provider's own error body (api_key_expired) verbatim.
// Neither outcome may fail the command.
func TestDoctorLLMEndpoint(t *testing.T) {
	blankEnv(t)
	t.Run("ok", func(t *testing.T) {
		var gotPath, gotAuth, gotCustom string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotCustom = r.Header.Get("x-custom-header")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
		}))
		defer srv.Close()

		path := filepath.Join(t.TempDir(), "config.toml")
		writeConfig(t, path,
			"[llm]\nprovider = \"openaicompat\"\napi_key_env = \"MYUL_TEST_KEY\"\n"+
				"base_url = "+strconv.Quote(srv.URL+"/v1/chat/completions")+"\n"+
				"[llm.headers]\nx-custom-header = \"custom-value\"\n")
		t.Setenv("MYUL_TEST_KEY", "sk-test")

		out, err := execute(t, "--config", path, "doctor")
		if err != nil {
			t.Fatalf("doctor error = %v (output %q)", err, out)
		}
		if !strings.Contains(out, "ok: LLM endpoint reachable") {
			t.Errorf("doctor output missing the reachable line: %q", out)
		}
		// full endpoint base normalises: /v1/models, not /v1/chat/completions/models
		if gotPath != "/v1/models" {
			t.Errorf("probe path = %q, want /v1/models", gotPath)
		}
		if gotAuth != "Bearer sk-test" {
			t.Errorf("probe Authorization = %q, want %q", gotAuth, "Bearer sk-test")
		}
		if gotCustom != "custom-value" {
			t.Errorf("probe x-custom-header = %q, want %q", gotCustom, "custom-value")
		}
	})

	t.Run("provider error body surfaces", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error",` +
				`"code":"api_key_expired","message":"This API key has expired..."}}`))
		}))
		defer srv.Close()

		path := filepath.Join(t.TempDir(), "config.toml")
		writeConfig(t, path,
			"[llm]\nprovider = \"openaicompat\"\napi_key_env = \"MYUL_TEST_KEY\"\n"+
				"base_url = "+strconv.Quote(srv.URL)+"\n")
		t.Setenv("MYUL_TEST_KEY", "sk-test")

		out, err := execute(t, "--config", path, "doctor")
		if err != nil {
			t.Fatalf("a failed endpoint probe must not fail doctor: %v (output %q)", err, out)
		}
		if !strings.Contains(out, "failed: 401 api_key_expired — This API key has expired...") {
			t.Errorf("doctor output missing the provider error summary: %q", out)
		}
	})
}

// TestDoctorLLMNoEndpointSkipped: with no provider or base_url there is
// nothing to probe — a skipped: line, never a failure.
func TestDoctorLLMNoEndpointSkipped(t *testing.T) {
	blankEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	writeConfig(t, path, "[llm]\n")

	out, err := execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v (output %q)", err, out)
	}
	if !strings.Contains(out, "skipped: no LLM endpoint configured") {
		t.Errorf("doctor output missing the skipped line: %q", out)
	}
}

// TestDoctorLLMModelNotInCatalog: a model missing from the embedded
// catalog for its provider gets an advisory note — catalogs go stale and
// model ids are free-form, so it never blocks or fails anything.
func TestDoctorLLMModelNotInCatalog(t *testing.T) {
	blankEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "config.toml")
	writeConfig(t, path,
		"[llm]\nprovider = \"opencode-go\"\nmodel = \"beam-free\"\napi_key_env = \"MYUL_TEST_KEY\"\n"+
			"base_url = "+strconv.Quote(srv.URL)+"\n")
	t.Setenv("MYUL_TEST_KEY", "sk-test")

	out, err := execute(t, "--config", path, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v (output %q)", err, out)
	}
	want := `note: model "beam-free" is not in the embedded catalog (may still work — catalogs can be stale)`
	if !strings.Contains(out, want) {
		t.Errorf("doctor output missing the catalog note: %q", out)
	}
}

// TestBareRootEmptyStore: with nothing to resume and no terminal, bare
// `myul` prints how to start a session and exits 1 — guidance, not a
// stack trace and not a hang.
func TestBareRootEmptyStore(t *testing.T) {
	dataDir, _ := isolatedDirs(t)

	out, err := execute(t)
	if err == nil {
		t.Fatalf("bare myul on an empty store: want error (exit 1), got nil (output %q)", out)
	}
	if !strings.Contains(err.Error(), "myul new") {
		t.Errorf("guidance missing `myul new`: %q", err)
	}
	if entries, rerr := os.ReadDir(filepath.Join(dataDir, "sessions")); rerr == nil && len(entries) > 0 {
		t.Errorf("bare myul created %d session(s) on an empty store", len(entries))
	}
}

// TestBareRootResumesNewestSession covers the headless bare-root path
// (--no-tui is its documented test/script seam): the most recently
// updated session is resumed and its id printed.
func TestBareRootResumesNewestSession(t *testing.T) {
	isolatedDirs(t)
	first := createSession(t, "First")
	// UpdatedAt stamps decide Store.List's order; separate them.
	time.Sleep(20 * time.Millisecond)
	second := createSession(t, "Second")

	out, err := execute(t, "--no-tui")
	if err != nil {
		t.Fatalf("bare myul --no-tui error = %v (output %q)", err, out)
	}
	if !strings.Contains(out, "resumed session "+second) {
		t.Errorf("bare myul --no-tui did not resume the newest session %q: %q", second, out)
	}
	if strings.Contains(out, "resumed session "+first) {
		t.Errorf("bare myul --no-tui resumed the older session %q: %q", first, out)
	}
}

// TestBareRootWithoutTerminalNeedsNoTUI: with one session but no
// --no-tui, a headless bare `myul` refuses to play instead of hanging.
// A model is configured so the refusal is the terminal check, not the
// configuration check.
func TestBareRootWithoutTerminalNeedsNoTUI(t *testing.T) {
	_, configDir := isolatedDirs(t)
	writeConfig(t, filepath.Join(configDir, "config.toml"),
		"[llm]\nprovider = \"openaicompat\"\nmodel = \"test-model\"\n")
	_ = createSession(t, "Only")

	out, err := execute(t)
	if err == nil {
		t.Fatalf("bare myul without a terminal: want error, got nil (output %q)", out)
	}
	if !strings.Contains(err.Error(), "interactive terminal is required") {
		t.Errorf("error should ask for --no-tui or a terminal: %q", err)
	}
}
