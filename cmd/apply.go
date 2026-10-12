package cmd

import (
	"fmt"

	"github.com/bkildow/wtx/internal/project"
	"github.com/bkildow/wtx/internal/ui"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "apply [name]",
		Short:             "Apply shared files to a worktree",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeWorktreeNames,
		RunE:              runApply,
	}
	cmd.Flags().Bool("all", false, "Apply to all worktrees")
	return cmd
}

func runApply(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	dry := IsDryRun()

	clone, err := openClone(ctx)
	if err != nil {
		return err
	}
	projectRoot, cfg := clone.Root(), clone.Config()

	wts, err := clone.Worktrees(ctx)
	if err != nil {
		return err
	}
	if err := rootApplyError(clone.Layout(), wts, args); err != nil {
		return err
	}

	if len(wts.Managed) == 0 {
		return fmt.Errorf("no worktrees found")
	}

	all, _ := cmd.Flags().GetBool("all")

	if all {
		include := includeSource(ctx, clone, wts)
		var totalResult project.ApplyResult
		for _, wt := range wts.Managed {
			ui.Step("Applying to: " + wt.Branch)
			vars := project.NewTemplateVars(projectRoot, wt.Path, wt.Branch)
			result, err := project.Apply(projectRoot, wt.Path, cfg, dry, &vars, include)
			if err != nil {
				return err
			}
			totalResult.Included += result.Included
			totalResult.Copied += result.Copied
			totalResult.Symlinked += result.Symlinked
		}
		ui.Success(fmt.Sprintf("Applied shared files to %d worktree(s) (%d included, %d copied, %d symlinked)",
			len(wts.Managed), totalResult.Included, totalResult.Copied, totalResult.Symlinked))
		return nil
	}

	selected, err := selectWorktree(args, wts.Managed)
	if err != nil {
		if ui.IsUserAbort(err) {
			return nil
		}
		return err
	}

	vars := project.NewTemplateVars(projectRoot, selected.Path, selected.Branch)
	result, err := project.Apply(projectRoot, selected.Path, cfg, dry, &vars, includeSource(ctx, clone, wts))
	if err != nil {
		return err
	}

	ui.Success(fmt.Sprintf("Applied shared files to: %s (%d included, %d copied, %d symlinked)",
		selected.Branch, result.Included, result.Copied, result.Symlinked))
	return nil
}

// rootApplyError explains why apply refuses the Project root when args names
// the branch it has checked out in a Checkout or In-repo layout: the root is
// the .worktreeinclude source, not a Managed worktree.
func rootApplyError(layout project.LayoutKind, wts project.Worktrees, args []string) error {
	if layout == project.BareLayout || !wts.HasMain || len(args) == 0 || wts.Main.Branch == "" || args[0] != wts.Main.Branch {
		return nil
	}
	if _, ok := findWorktreeByBranch(wts.Managed, args[0]); ok {
		return nil
	}
	return fmt.Errorf("%s is checked out at the project root, the .worktreeinclude source; wtx apply only targets managed worktrees", args[0])
}
