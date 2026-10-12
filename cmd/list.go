package cmd

import (
	"github.com/bkildow/wtx/internal/project"
	"github.com/bkildow/wtx/internal/ui"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all worktrees",
		Args:  cobra.NoArgs,
		RunE:  runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	clone, err := openClone(cmd.Context())
	if err != nil {
		return err
	}
	projectRoot := clone.Root()

	wts, err := clone.Worktrees(cmd.Context())
	if err != nil {
		return err
	}
	// In a Checkout or In-repo layout the Main worktree is the Project root,
	// not a Managed worktree, so it gets its own row.
	rootRow := clone.Layout() != project.BareLayout && wts.HasMain

	if len(wts.Managed) == 0 && !rootRow {
		ui.Info("No worktrees found. Use 'wtx add' to create one.")
		return nil
	}

	ui.Heading("Worktrees")

	t := ui.NewTable().Headers("BRANCH", "PATH", "COMMIT")
	if rootRow {
		branch := wts.Main.Branch
		if branch == "" {
			branch = "(detached)"
		}
		t.Row(branch, ui.DisplayPath("", projectRoot)+" (project root)", shortHead(wts.Main.Head))
	}
	paths := ui.NewPathDisplay(projectRoot)
	for _, wt := range wts.Managed {
		t.Row(wt.Branch, paths.Path(wt.Path), shortHead(wt.Head))
	}
	ui.PrintTable(t)
	return nil
}

// shortHead abbreviates a commit hash to 7 characters.
func shortHead(head string) string {
	if len(head) > 7 {
		return head[:7]
	}
	return head
}
