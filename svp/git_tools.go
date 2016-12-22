package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	curBranch = "" // cache current branch
	gitRoot   = "" // cache absolute path to git root
)

/* Print the name of the current branch of the git repo you're in
 */
func CurBranch() (string, error) {
	// Use cached result if available
	if len(curBranch) > 0 {
		return curBranch, nil
	}

	// Get current branch
	cmdString := "git rev-parse --abbrev-ref HEAD"
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	curBranch, err := branchCmd.Output()
	if err != nil {
		return "", fmt.Errorf("Could not get current git branch (\"%s\"):\n%s",
			cmdString, err)
	}
	return strings.TrimSpace(string(curBranch)), nil
}

/* Print the absolute path to the root of the git repo you're in
 */
func GitRoot() (string, error) {
	// Use cached result if available
	if len(gitRoot) > 0 {
		return gitRoot, nil
	}

	// Get current branch
	cmdString := "git rev-parse --show-toplevel"
	getRootCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	gitRoot, err := getRootCmd.Output()
	if err != nil {
		return "", fmt.Errorf("Could not get root of current git repo (\"%s\"):\n%s",
			cmdString, err)
	}
	return strings.TrimSpace(string(gitRoot)), nil
}

/* Cobra commands that print the outputs of CurBranch() and GitRoot()
 */
func GitHelperCommands() []*cobra.Command {
	return []*cobra.Command{
		&cobra.Command{
			Use:   "cur-branch",
			Short: "Print the name of the current branch of the git repo you're in",
			Run: boundedCommand(0, 0, func(args []string) error {
				branch, err := CurBranch()
				if err != nil {
					return err
				}
				fmt.Println(branch)
				return nil
			}),
		},
		&cobra.Command{
			Use:   "root-path",
			Short: "Print absolute path to the root of the git repo you're in",
			Run: boundedCommand(0, 0, func(args []string) error {
				root, err := GitRoot()
				if err != nil {
					return err
				}
				fmt.Println(root)
				return nil
			}),
		},
	}
}
