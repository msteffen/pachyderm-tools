package main

import (
	"fmt"
	// "os"

	"github.com/spf13/cobra"
)

func boundedCommand(minargs, maxargs int, f func([]string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if minargs > maxargs {
			panic(fmt.Sprintf("Invalid arguments to 'boundedCommand': 'minargs' must be <= 'maxargs', but got %d > %d", minargs, maxargs))
		} else if minargs == maxargs {
			if len(args) != minargs {
				panic(fmt.Sprintf("Expected exactly %d arguments, but got %d", minargs, len(args)))
			}
		} else {
			if len(args) < minargs {
				panic(fmt.Sprintf("Expected at least %d arguments, but got %d", minargs, len(args)))
			} else if len(args) > maxargs {
				panic(fmt.Sprintf("Expected at most %d arguments, but got %d", maxargs, len(args)))
			}
		}
		err := f(args)
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	root := &cobra.Command{
		Use: "svp <command>",
	}

	root.AddCommand(DiffCommand())
	root.AddCommand(ChangedFilesCommand())
	for _, cmd := range GitHelperCommands() {
		root.AddCommand(cmd)
	}
	root.Execute()
}
