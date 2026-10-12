package cmd

import (
	"context"
	"os"

	"github.com/bkildow/wtx/internal/project"
	"github.com/spf13/cobra"
)

// completeWorktreeNames completes a worktree argument with the Managed
// worktrees' branches.
func completeWorktreeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeWorktrees(cmd, args, false)
}

// completeAllWorktreeNames is completeWorktreeNames plus the Project root's
// branch when the root is a worktree (see project.Worktrees.All).
func completeAllWorktreeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeWorktrees(cmd, args, true)
}

func completeWorktrees(cmd *cobra.Command, args []string, withRoot bool) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names, err := listWorktreeNames(cmd.Context(), withRoot)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func listWorktreeNames(ctx context.Context, withRoot bool) ([]string, error) {
	clone, err := completionClone(ctx)
	if err != nil {
		return nil, err
	}

	wts, err := clone.Worktrees(ctx)
	if err != nil {
		return nil, err
	}
	filtered := wts.Managed
	if withRoot {
		filtered = wts.All()
	}

	names := make([]string, 0, len(filtered)+1)
	names = append(names, ".")
	for _, wt := range filtered {
		names = append(names, wt.Branch)
	}

	return names, nil
}

// completeBranchNames completes flag values with the project's remote branches.
func completeBranchNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	clone, err := completionClone(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	branches, err := clone.Runner().ListRemoteBranches(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return branches, cobra.ShellCompDirectiveNoFileComp
}

// completionClone opens the Clone containing the working directory for shell
// completion functions. Unlike openClone it prints nothing.
func completionClone(ctx context.Context) (*project.Clone, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return project.Open(ctx, cwd, project.Options{})
}
