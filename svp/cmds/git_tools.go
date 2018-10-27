package cmds

import (
	"fmt"

	"github.com/msteffen/pachyderm-tools/svp/git"

	"github.com/spf13/cobra"
)

// GitHelperCommands returns Cobra commands that print the outputs of
// CurBranch() and GitRoot()
func GitHelperCommands() []*cobra.Command {
	return []*cobra.Command{
		&cobra.Command{
			Use:   "cur-branch",
			Short: "Print the name of the current branch of the git repo you're in",
			Run: BoundedCommand(0, 0, func(args []string) error {
				if git.Root == "" {
					return fmt.Errorf("cur-branch must be run from inside a git repo")
				}
				fmt.Println(git.CurBranch)
				return nil
			}),
		},
		&cobra.Command{
			Use:   "root-path",
			Short: "Print absolute path to the root of the git repo you're in",
			Run: BoundedCommand(0, 0, func(args []string) error {
				if git.Root == "" {
					return fmt.Errorf("root-path must be run from inside a git repo")
				}
				fmt.Println(git.Root)
				return nil
			}),
		},
	}
}
