package main

import (
	"github.com/msteffen/pachyderm-tools/svp/cmds"

	"github.com/spf13/cobra"
)

// RootCmd returns the root cobra command (off of which all other svp commands
// branch).
func RootCmd() *cobra.Command {
	// Generate root cobra command & return it
	root := &cobra.Command{
		Use: "svp <command>",
	}
	root.AddCommand(cmds.DiffCommand())
	root.AddCommand(cmds.ChangedFilesCommand())
	for _, cmd := range cmds.GitHelperCommands() {
		root.AddCommand(cmd)
	}
	for _, cmd := range cmds.ClientCommands() {
		root.AddCommand(cmd)
	}
	return root
}

func main() {
	RootCmd().Execute()
}
