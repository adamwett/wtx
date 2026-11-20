package cmd

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/bkildow/wtx/internal/project"
	"github.com/bkildow/wtx/internal/ui"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [name]",
		Short: "Apply shared files to a worktree",
		Long: `Apply shared files to an existing worktree.

Copies .worktreeinclude and shared/copy files the worktree is missing and
links shared/symlink entries. Files the worktree already has are kept, so a
populated vendor/ or node_modules/ is never reset to an older shared copy.
Name a path with --overwrite to replace it, or pass --force to replace every
shared/copy file.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeWorktreeNames,
		RunE:              runApply,
	}
	cmd.Flags().Bool("all", false, "Apply to all worktrees")
	cmd.Flags().Bool("force", false, "Replace every shared/copy file the worktree already has")
	cmd.Flags().StringArray("overwrite", nil, "Replace this shared/copy path (file or directory, relative to the worktree root) if the worktree has it; repeatable")
	cmd.MarkFlagsMutuallyExclusive("force", "overwrite")
	_ = cmd.RegisterFlagCompletionFunc("overwrite", completeOverwritePaths)
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

	mode, err := applyCopyMode(cmd, filepath.Join(clone.SharedDir(), "copy"))
	if err != nil {
		return err
	}

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
		paths := ui.NewPathDisplay(projectRoot)
		applied, failed := 0, 0
		for _, wt := range wts.Managed {
			// A detached worktree has no branch to name it by.
			name := cmp.Or(wt.Branch, paths.Path(wt.Path))
			if wt.Prunable {
				// Applying would recreate the deleted directory as a plain
				// folder of shared files.
				ui.Warning(fmt.Sprintf("Skipping %s: its directory %s is missing", name, paths.Path(wt.Path)))
				continue
			}
			ui.Step("Applying to: " + name)
			vars := project.NewTemplateVars(projectRoot, wt.Path, wt.Branch)
			result, err := project.Apply(projectRoot, wt.Path, cfg, dry, &vars, include, mode)
			if err != nil {
				// Carry on, so one bad worktree never leaves the rest without
				// their shared files.
				ui.Error(fmt.Sprintf("Could not apply to %s: %v", name, err))
				failed++
				continue
			}
			applied++
			totalResult.Included += result.Included
			totalResult.Copied += result.Copied
			totalResult.Kept += result.Kept
			totalResult.Symlinked += result.Symlinked
		}
		reportApply(fmt.Sprintf("Applied shared files to %d of %d worktree(s)", applied, len(wts.Managed)), totalResult, mode)
		if failed > 0 {
			return fmt.Errorf("apply failed for %d worktree(s): %w", failed, ErrReported)
		}
		return nil
	}

	selected, err := selectWorktree(args, wts.Managed)
	if err != nil {
		if ui.IsUserAbort(err) {
			return nil
		}
		return err
	}

	name := cmp.Or(selected.Branch, ui.DisplayPath(projectRoot, selected.Path))
	if selected.Prunable {
		return fmt.Errorf("%s: its directory %s is missing", name, ui.DisplayPath(projectRoot, selected.Path))
	}
	vars := project.NewTemplateVars(projectRoot, selected.Path, selected.Branch)
	result, err := project.Apply(projectRoot, selected.Path, cfg, dry, &vars, includeSource(ctx, clone, wts), mode)
	if err != nil {
		return err
	}

	reportApply("Applied shared files to: "+name, result, mode)
	return nil
}

// applyCopyMode reads --force and --overwrite: apply keeps the shared/copy
// files a worktree already has unless one of them says otherwise.
func applyCopyMode(cmd *cobra.Command, copyDir string) (project.CopyMode, error) {
	if force, _ := cmd.Flags().GetBool("force"); force {
		return project.ReplaceExisting, nil
	}
	paths, _ := cmd.Flags().GetStringArray("overwrite")
	replace, err := project.ReplacePaths(copyDir, paths)
	if err != nil {
		return project.CopyMode{}, fmt.Errorf("--overwrite %w", err)
	}
	return project.CopyMode{Seed: true, Replace: replace}, nil
}

// completeOverwritePaths completes --overwrite with the shared/copy entries,
// one directory level at a time, naming each .template file by the path it
// renders to.
func completeOverwritePaths(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clone, err := completionClone(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	dir, _ := path.Split(toComplete)
	if dir != "" && !filepath.IsLocal(dir) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := os.ReadDir(filepath.Join(clone.SharedDir(), "copy", filepath.FromSlash(dir)))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	directive := cobra.ShellCompDirectiveNoFileComp
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		name := dir + e.Name()
		if e.IsDir() {
			// Leave the cursor after the slash so the next TAB goes inside.
			name += "/"
			directive |= cobra.ShellCompDirectiveNoSpace
		} else {
			name = project.StripTemplateExt(name)
		}
		if !slices.Contains(paths, name) {
			paths = append(paths, name)
		}
	}
	return paths, directive
}

// reportApply prints the summary line, and how to replace the files apply
// kept unless mode already replaced every one it could.
func reportApply(prefix string, r project.ApplyResult, mode project.CopyMode) {
	kept := ""
	if r.Kept > 0 {
		kept = fmt.Sprintf(", %d kept", r.Kept)
	}
	ui.Success(fmt.Sprintf("%s (%d included, %d copied%s, %d symlinked)", prefix, r.Included, r.Copied, kept, r.Symlinked))
	if r.Kept > 0 && mode.Seed {
		ui.Info("Shared files the worktree already had were kept; replace them with --overwrite <path> or --force")
	}
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
