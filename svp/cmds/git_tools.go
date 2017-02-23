package cmds

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Information about this client's git repo. If 'svp' is run outside a git
	// repo, both of these will be the empty string
	CurBranch string
	GitRoot   string

	// This error (from the 'git' CLI) means 'svp' was not run from a git repo
	/*const */ notAGitRepo = regexp.MustCompile("^fatal: Not a git repository")
)

// CurBranch returns the name of the current branch of the git repo you're in
func initCurBranch() error {
	// Get current branch
	op := StartOp()
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
	op := StartOp()
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

func InitGitInfo() error {
	var err error = nil
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
			Run: boundedCommand(0, 0, func(args []string) error {
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
			Run: boundedCommand(0, 0, func(args []string) error {
				if GitRoot == "" {
					return fmt.Errorf("root-path must be run from inside a git repo")
				}
				fmt.Println(GitRoot)
				return nil
			}),
		},
	}
}
