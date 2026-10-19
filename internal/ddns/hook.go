package ddns

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// hookTimeout stops a Success Hook or Failure Hook that hangs, so it cannot
// block the following update checks. A variable so tests can shorten it.
var hookTimeout = 5 * time.Minute

// hookEvent is what a hook is told about the update check that triggered it.
type hookEvent struct {
	PublicIP      string
	PreviousIP    string
	FailureReason string
}

func (e hookEvent) env() []string {
	env := []string{
		"PUBLIC_IP=" + e.PublicIP,
		"PREVIOUS_IP=" + e.PreviousIP,
		// Deprecated: the original shell script exposed its PublicIP variable
		// to hooks. Kept for 1.x, removed in 2.0.0.
		"PublicIP=" + e.PublicIP,
	}
	if e.FailureReason != "" {
		env = append(env, "FAILURE_REASON="+e.FailureReason)
	}
	return env
}

// runHook runs command with sh -c. A failing or timed-out hook is logged and
// otherwise ignored.
func runHook(ctx context.Context, name, command string, event hookEvent) {
	if command == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Env = append(os.Environ(), event.env()...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	// Run the hook in its own process group and kill the whole group on
	// timeout or shutdown, so commands started by the shell stop too.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone // the hook exited just before the deadline
		}
		return err
	}
	log.Printf("running %s", name)
	if err := cmd.Run(); err != nil {
		log.Printf("%s failed: %v", name, err)
	}
}
