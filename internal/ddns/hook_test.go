package ddns

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestRunHookPassesEventEnvironment(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env")
	t.Setenv("HOOK_OUT", out)
	t.Setenv("DOMAIN", "")
	event := hookEvent{PublicIP: "198.51.100.4", PreviousIP: "203.0.113.7", FailureReason: "boom"}

	runHook(context.Background(), "failure hook",
		`printf '%s|%s|%s|%s|%s' "$PUBLIC_IP" "$PREVIOUS_IP" "$FAILURE_REASON" "$PublicIP" "$DOMAIN" > "$HOOK_OUT"`,
		event)

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("hook did not run: %v", err)
	}
	if want := "198.51.100.4|203.0.113.7|boom|198.51.100.4|"; string(got) != want {
		t.Errorf("hook saw %q, want %q", got, want)
	}
}

func TestRunHookInheritsEnvironment(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env")
	t.Setenv("HOOK_OUT", out)
	t.Setenv("DOMAIN", "example.com")

	runHook(context.Background(), "success hook", `printf '%s' "$DOMAIN" > "$HOOK_OUT"`, hookEvent{})

	if got, _ := os.ReadFile(out); string(got) != "example.com" {
		t.Errorf("hook saw DOMAIN=%q", got)
	}
}

func TestRunHookOmitsFailureReasonOnSuccess(t *testing.T) {
	if env := (hookEvent{PublicIP: "198.51.100.4"}).env(); strings.Contains(strings.Join(env, "\n"), "FAILURE_REASON") {
		t.Errorf("success hook env = %v, want no FAILURE_REASON", env)
	}
}

func TestRunHookEmptyCommandIsNoop(t *testing.T) {
	logs := captureLog(t)
	runHook(context.Background(), "success hook", "", hookEvent{})
	if logs.Len() != 0 {
		t.Errorf("logged %q for an empty hook", logs.String())
	}
}

func TestRunHookFailureIsLogged(t *testing.T) {
	logs := captureLog(t)
	runHook(context.Background(), "success hook", "exit 3", hookEvent{})
	if !strings.Contains(logs.String(), "success hook failed: exit status 3") {
		t.Errorf("logs = %q", logs.String())
	}
}

func TestRunHookTimeoutKillsChildProcesses(t *testing.T) {
	defer func(d time.Duration) { hookTimeout = d }(hookTimeout)
	hookTimeout = 300 * time.Millisecond
	pidFile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("PID_FILE", pidFile)
	logs := captureLog(t)

	start := time.Now()
	runHook(context.Background(), "success hook", `sleep 30 & echo $! > "$PID_FILE"; wait`, hookEvent{})
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("runHook returned after %v, want shortly after the timeout", elapsed)
	}
	if !strings.Contains(logs.String(), "success hook failed") {
		t.Errorf("logs = %q, want the timeout reported", logs.String())
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("hook did not start its child: %v", err)
	}
	var pid int
	if _, err := fmt.Sscan(string(data), &pid); err != nil {
		t.Fatalf("pid file %q: %v", data, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("child process %d of the hook survived the timeout", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// processAlive reports whether pid is running. A zombie counts as stopped: it
// only waits for whatever reaps orphans here, which may not be prompt when the
// test itself runs as PID 1 in a container.
func processAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true // no procfs (macOS): trust kill(0)
	}
	// The state follows the parenthesised command name.
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && i+2 < len(s) {
		return s[i+2] != 'Z'
	}
	return true
}
