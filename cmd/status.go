package cmd

import (
	"cmp"
	"strings"

	"github.com/bkildow/wtx/internal/project"
	"github.com/bkildow/wtx/internal/ui"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of all worktrees",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	clone, err := openClone(ctx)
	if err != nil {
		return err
	}
	projectRoot, cfg := clone.Root(), clone.Config()

	runner := clone.Runner()
	filtered, err := clone.ManagedWorktrees(ctx)
	if err != nil {
		return err
	}

	if len(filtered) == 0 {
		ui.Info("No worktrees found. Use 'wtx add' to create one.")
		warnLowDisk(projectRoot, cfg)
		return nil
	}

	ui.Heading("Worktree Status")

	t := ui.NewTable().Headers("BRANCH", "PATH", "COMMIT", "STATUS", "SETUP", "LAST COMMIT")
	paths := ui.NewPathDisplay(projectRoot)
	var hints []string
	for _, wt := range filtered {
		relPath := paths.Path(wt.Path)

		shortHead := wt.Head
		if len(shortHead) > 7 {
			shortHead = shortHead[:7]
		}

		// An unreadable worktree gets an error row and a hint rather than
		// hiding the rest of the table.
		styledStatus := ui.StyleSuccess.Render("clean")
		dirty, dirtyErr := runner.IsWorktreeDirty(ctx, wt.Path)
		switch {
		case dirtyErr != nil:
			styledStatus = ui.StyleError.Render("error")
		case dirty:
			styledStatus = ui.StyleWarning.Render("dirty")
		}

		age, ageErr := runner.GetLastCommitAge(ctx, wt.Path)
		if ageErr != nil {
			age = ui.StyleError.Render("error")
		}

		if gitErr := cmp.Or(dirtyErr, ageErr); gitErr != nil {
			hints = append(hints, gitErrorHint(wt.Branch, gitErr))
		}

		// An unreadable state renders as "-", like a missing one.
		state, _ := project.ResolveSetupStatus(wt.Path)
		styledSetup := renderSetupStatus(state)
		if state != nil && state.Status == project.SetupFailed {
			hints = append(hints, setupFailureHint(wt.Branch, wt.Path, state))
		}

		t.Row(wt.Branch, relPath, shortHead, styledStatus, styledSetup, age)
	}
	ui.PrintTable(t)
	for _, hint := range hints {
		ui.Warning(hint)
	}
	warnLowDisk(projectRoot, cfg)
	return nil
}

// setupFailureHint tells the user where to look after a failed setup. Only a
// background run has a log; a foreground run printed its output to the terminal.
func setupFailureHint(branch, worktreePath string, state *project.SetupState) string {
	hint := "Setup failed for " + branch
	if firstLine, _, _ := strings.Cut(state.Error, "\n"); firstLine != "" {
		hint += ": " + firstLine
	}
	if project.SetupLog(worktreePath, state) != "" {
		return hint + " — run 'wtx logs " + branch + "'"
	}
	return hint + " — re-run with 'wtx setup " + branch + " --foreground'"
}

// gitErrorHint explains an error row in one line. Runner errors put the
// command first and git's stderr after it, so a "fatal:" line is preferred.
func gitErrorHint(branch string, err error) string {
	msg := err.Error()
	reason, _, _ := strings.Cut(msg, "\n")
	for line := range strings.Lines(msg) {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "fatal:") {
			reason = line
			break
		}
	}
	return "Could not read status for " + branch + ": " + reason
}

func renderSetupStatus(state *project.SetupState) string {
	if state == nil {
		return ui.StyleMuted.Render("-")
	}

	switch state.Status {
	case project.SetupRunning:
		return ui.StyleInfo.Render("In Progress")
	case project.SetupComplete:
		return ui.StyleSuccess.Render("Complete")
	case project.SetupSkipped:
		return ui.StyleWarning.Render("Skipped")
	case project.SetupFailed:
		return ui.StyleError.Render("Failed")
	default:
		return ui.StyleMuted.Render("-")
	}
}
