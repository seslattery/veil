package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// Sandbox executes commands in a macOS seatbelt sandbox.
type Sandbox struct {
	ProxyPort         int
	AllowedWritePaths []string
}

// New creates a new Sandbox.
func New(proxyPort int, allowedWritePaths []string) *Sandbox {
	return &Sandbox{
		ProxyPort:         proxyPort,
		AllowedWritePaths: allowedWritePaths,
	}
}

// Run executes the command in the sandbox.
// If stdin is a terminal, it allocates a PTY for interactive use.
func (s *Sandbox) Run(ctx context.Context, name string, args []string, env []string) error {
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	profile, err := GenerateProfile(s.ProxyPort, s.AllowedWritePaths, isTTY)
	if err != nil {
		return fmt.Errorf("generating profile: %w", err)
	}

	// Write profile to temp file (more reliable than -p inline)
	profileFile, err := os.CreateTemp("", "veil-seatbelt-*.sb")
	if err != nil {
		return fmt.Errorf("creating profile file: %w", err)
	}
	profilePath := profileFile.Name()
	defer os.Remove(profilePath)

	if _, err := profileFile.WriteString(profile); err != nil {
		return fmt.Errorf("writing profile: %w", err)
	}
	if err := profileFile.Close(); err != nil {
		return fmt.Errorf("closing profile file: %w", err)
	}

	sandboxArgs := []string{"-f", profilePath, name}
	sandboxArgs = append(sandboxArgs, args...)

	cmd := exec.CommandContext(ctx, "sandbox-exec", sandboxArgs...)
	cmd.Env = env

	if isTTY {
		return s.runWithPTY(cmd)
	}
	return s.runWithPipes(cmd)
}

// Profile returns the generated seatbelt profile for dry-run display.
func (s *Sandbox) Profile() (string, error) {
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	return GenerateProfile(s.ProxyPort, s.AllowedWritePaths, isTTY)
}

func (s *Sandbox) runWithPTY(cmd *exec.Cmd) error {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("starting pty: %w", err)
	}

	// Done channel to signal goroutines to exit
	done := make(chan struct{})

	// Handle terminal resize
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-done:
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
					// Ignore resize errors
				}
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize

	// Set stdin to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// Not a terminal, continue without raw mode
		oldState = nil
	}

	// Copy stdin to pty
	go func() {
		io.Copy(ptmx, os.Stdin)
	}()

	// Copy pty to stdout in goroutine
	go func() {
		io.Copy(os.Stdout, ptmx)
	}()

	// Wait for process to complete
	err = cmd.Wait()

	// Signal goroutines to stop
	close(done)
	signal.Stop(ch)
	ptmx.Close()

	if oldState != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
	}

	return err
}

func (s *Sandbox) runWithPipes(cmd *exec.Cmd) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
