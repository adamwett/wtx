package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bkildow/wtx/internal/project"
)

func TestSetupFailureHint(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, project.SetupLogFile)
	if err := os.WriteFile(logPath, []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		state *project.SetupState
		want  string
	}{
		{
			name:  "background failure points at the log",
			state: &project.SetupState{Status: project.SetupFailed, Error: "hook 'npm ci' failed: exit status 1", LogFile: logPath},
			want:  "Setup failed for feat: hook 'npm ci' failed: exit status 1 — run 'wtx logs feat'",
		},
		{
			name:  "multi-line error keeps the first line",
			state: &project.SetupState{Status: project.SetupFailed, Error: "first\nsecond", LogFile: logPath},
			want:  "Setup failed for feat: first — run 'wtx logs feat'",
		},
		{
			name:  "empty error",
			state: &project.SetupState{Status: project.SetupFailed, LogFile: logPath},
			want:  "Setup failed for feat — run 'wtx logs feat'",
		},
		{
			name:  "foreground failure suggests a re-run",
			state: &project.SetupState{Status: project.SetupFailed, Error: "boom"},
			want:  "Setup failed for feat: boom — re-run with 'wtx setup feat --foreground'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setupFailureHint("feat", dir, tt.state); got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestStatusSetupFailureHint(t *testing.T) {
	// runStatusWith writes the given setup state and a log, runs status, and
	// returns its output.
	runStatusWith := func(t *testing.T, state *project.SetupState) string {
		t.Helper()
		worktree := newSetupProject(t)
		t.Setenv("WTX_NO_DISK_WARN", "1")
		state.LogFile = project.SetupLogPath(worktree)
		writeSetupFixture(t, worktree, state, "output\n")
		out := captureUI(t)

		command := newStatusCmd()
		command.SetContext(context.Background())
		if err := runStatus(command, nil); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}

	t.Run("failed", func(t *testing.T) {
		out := runStatusWith(t, &project.SetupState{Status: project.SetupFailed, Error: "hook failed"})
		if !strings.Contains(out, "wtx logs feature") {
			t.Errorf("missing failure hint:\n%s", out)
		}
	})

	t.Run("complete", func(t *testing.T) {
		out := runStatusWith(t, &project.SetupState{Status: project.SetupComplete})
		if strings.Contains(out, "Setup failed") {
			t.Errorf("unexpected failure hint:\n%s", out)
		}
	})
}

func TestGitErrorHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "fatal line in stderr wins",
			err:  errors.New("git -C /x status --porcelain: exit status 128\nfatal: gitfile does not point to a valid repository\n"),
			want: "Could not read status for feat: fatal: gitfile does not point to a valid repository",
		},
		{
			name: "no fatal line keeps the first line",
			err:  errors.New("git -C /x log -1: exit status 1\nerror: something\n"),
			want: "Could not read status for feat: git -C /x log -1: exit status 1",
		},
		{
			name: "single line",
			err:  errors.New("boom"),
			want: "Could not read status for feat: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitErrorHint("feat", tt.err); got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestStatusBrokenWorktree(t *testing.T) {
	worktree := newSetupProject(t)
	root := filepath.Dir(filepath.Dir(worktree))
	broken := filepath.Join(root, "worktrees", "alpha")
	if out, err := exec.Command("git", "-C", root, "worktree", "add", "-b", "alpha", broken).CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: /nonexistent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WTX_NO_DISK_WARN", "1")
	out := captureUI(t)

	command := newStatusCmd()
	command.SetContext(context.Background())
	if err := runStatus(command, nil); err != nil {
		t.Fatalf("status should not fail on one broken worktree: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Could not read status for alpha: fatal:") {
		t.Errorf("missing broken-worktree hint:\n%s", got)
	}
	rows := map[string]string{"alpha": "error", "feature": "clean"}
	for branch, want := range rows {
		found := false
		for line := range strings.Lines(got) {
			if strings.Contains(line, branch) && !strings.Contains(line, "Could not read") {
				found = true
				if !strings.Contains(line, want) {
					t.Errorf("%s row should show %q: %q", branch, want, line)
				}
			}
		}
		if !found {
			t.Errorf("missing %s row:\n%s", branch, got)
		}
	}
}
