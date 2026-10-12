package cmd

import (
	"path/filepath"
	"testing"

	"github.com/bkildow/wtx/internal/git"
	"github.com/bkildow/wtx/internal/project"
)

func TestRootApplyError(t *testing.T) {
	root := filepath.FromSlash("/proj")
	rootMain := git.WorktreeInfo{Path: root, Branch: "main"}
	feature := git.WorktreeInfo{Path: filepath.FromSlash("/elsewhere/feature"), Branch: "feature"}
	mainWT := git.WorktreeInfo{Path: filepath.Join(root, "worktrees", "main"), Branch: "main"}
	checkout := project.ClassifyWorktrees([]git.WorktreeInfo{rootMain, feature}, root, project.CheckoutLayout, "main")
	detached := project.ClassifyWorktrees([]git.WorktreeInfo{{Path: root}, feature}, root, project.CheckoutLayout, "main")
	bare := project.ClassifyWorktrees([]git.WorktreeInfo{mainWT, feature}, root, project.BareLayout, "main")

	tests := []struct {
		name    string
		layout  project.LayoutKind
		wts     project.Worktrees
		args    []string
		wantErr bool
	}{
		{"checkout: the root's branch", project.CheckoutLayout, checkout, []string{"main"}, true},
		{"in-repo: the root's branch", project.InRepoLayout, checkout, []string{"main"}, true},
		{"checkout: a Managed worktree", project.CheckoutLayout, checkout, []string{"feature"}, false},
		{"checkout: no argument", project.CheckoutLayout, checkout, nil, false},
		{"checkout: detached root", project.CheckoutLayout, detached, []string{""}, false},
		{"bare: main is Managed", project.BareLayout, bare, []string{"main"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rootApplyError(tt.layout, tt.wts, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("rootApplyError = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
