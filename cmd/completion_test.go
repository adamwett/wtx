package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// completesFiles lists the arguments ("<command>") and flags
// ("<command> --<flag>") that take a file path, so the shell's file
// completion fits them, each with what it names.
var completesFiles = map[string]string{
	"share":                "a machine-local file to share",
	"claude init --binary": "the wtx binary hooks run",
	"herdr init --binary":  "the wtx binary herdr runs",
}

// TestEveryCommandDeclaresCompletion fails when a visible command's arguments
// or non-bool flag values fall back to completing files without being listed
// in completesFiles, so TAB never offers files where none fit.
func TestEveryCommandDeclaresCompletion(t *testing.T) {
	seen := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			walk(sub)
		}
		if !c.Runnable() || c.Hidden || c == rootCmd {
			return
		}
		path := strings.TrimPrefix(c.CommandPath(), rootCmd.Name()+" ")
		seen[path] = true
		if c.ValidArgs == nil && c.ValidArgsFunction == nil && completesFiles[path] == "" {
			t.Errorf("'wtx %s' completes its arguments as files: set ValidArgsFunction (cobra.NoFileCompletions when it takes none), or list it in completesFiles", path)
		}
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden || f.Value.Type() == "bool" {
				return
			}
			key := path + " --" + f.Name
			seen[key] = true
			if _, ok := c.GetFlagCompletionFunc(f.Name); !ok && completesFiles[key] == "" {
				t.Errorf("'wtx %s' completes its value as files: register a completion (cobra.NoFileCompletions for free text), or list it in completesFiles", key)
			}
		})
	}
	walk(rootCmd)

	for key := range completesFiles {
		if !seen[key] {
			t.Errorf("completesFiles lists %q, which is no longer a visible command or flag; delete the entry", key)
		}
	}
}
