package cmds

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Information about this client's git repo
	CurBranch string
	GitRoot   string
)

// CurBranch returns the name of the current branch of the git repo you're in
func initCurBranch() error {
	// Use cached result if available
	if len(CurBranch) > 0 {
		return nil
	}

	// Get current branch
	cmdString := "git rev-parse --abbrev-ref HEAD"
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	curBranch, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("could not get current git branch (cmd: \"%s\"):\n%s",
			cmdString, err)
	}
	CurBranch = strings.TrimSpace(string(curBranch))
	return nil
}

// GitRoot returns the absolute path to the root of the git repo you're in
func initGitRoot() error {
	// Use cached result if available
	if len(GitRoot) > 0 {
		return nil
	}
	cmdString := "git rev-parse --show-toplevel"
	getRootCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	gitRoot, err := getRootCmd.Output()
	if err != nil {
		return fmt.Errorf("could not get root of current git repo (cmd: \"%s\"):"+
			"\n%s", cmdString, err)
	}
	GitRoot = strings.TrimSpace(string(gitRoot))
	return nil
}

func InitGitInfo() error {
	var err error = nil
	for _, f := range []func() error{initCurBranch, initGitRoot} {
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
				fmt.Println(CurBranch)
				return nil
			}),
		},
		&cobra.Command{
			Use:   "root-path",
			Short: "Print absolute path to the root of the git repo you're in",
			Run: boundedCommand(0, 0, func(args []string) error {
				fmt.Println(GitRoot)
				return nil
			}),
		},
	}
}
