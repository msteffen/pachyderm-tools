package cmds

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/msteffen/pachyderm-tools/op"
	"github.com/spf13/cobra"
)

var (
	// CurBranch is the name of the git branch that is currently checked out
	CurBranch string
	// GitRoot is the path to the root of the current git repo
	GitRoot string

	// This error (from the 'git' CLI) means 'svp' was not run from a git repo
	/*const */
	notAGitRepo = regexp.MustCompile("^fatal: Not a git repository")
)

// CurBranch returns the name of the current branch of the git repo you're in
func initCurBranch() error {
	// Get current branch
	op := op.StartOp()
	op.CollectStdOut()
	op.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if op.LastError() != nil {
		return fmt.Errorf("could not get current branch of git repo:\n%s",
			op.DetailedError())
	}
	CurBranch = strings.TrimSpace(op.Output())
	return nil
}

// GitRoot returns the absolute path to the root of the git repo you're in
func initGitRoot() error {
	op := op.StartOp()
	op.CollectStdOut()
	op.Run("git", "rev-parse", "--show-toplevel")
	if op.LastError() != nil {
		if notAGitRepo.Match(op.LastErrorMsg()) {
			GitRoot = "" // no error, but we're not in a git repo
			return nil
		}
		return fmt.Errorf("could not get root of git repo:\n%s", op.DetailedError())
	}
	GitRoot = strings.TrimSpace(op.Output())
	return nil
}

// InitGitInfo detects if svp is being run from inside a git repo, and if so,
// runs a collection of git commands to learn about it.
func InitGitInfo() error {
	var err error
	for i, f := range []func() error{initGitRoot, initCurBranch} {
		if i > 0 && GitRoot == "" {
			break // we're not in a git repo; quit early
		}
		err = f()
		if err != nil {
			break
		}
	}
	return err
}

// GitHelperCommands returns Cobra commands that print the outputs of
// CurBranch() and GitRoot()
func GitHelperCommands() []*cobra.Command {
	return []*cobra.Command{
		&cobra.Command{
			Use:   "cur-branch",
			Short: "Print the name of the current branch of the git repo you're in",
			Run: BoundedCommand(0, 0, func(args []string) error {
				if GitRoot == "" {
					return fmt.Errorf("cur-branch must be run from inside a git repo")
				}
				fmt.Println(CurBranch)
				return nil
			}),
		},
		&cobra.Command{
			Use:   "root-path",
			Short: "Print absolute path to the root of the git repo you're in",
			Run: BoundedCommand(0, 0, func(args []string) error {
				if GitRoot == "" {
					return fmt.Errorf("root-path must be run from inside a git repo")
				}
				fmt.Println(GitRoot)
				return nil
			}),
		},
	}
}
