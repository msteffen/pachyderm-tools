package cmds

import (
	"fmt"
	"os"
	"path"
	"regexp"

	"github.com/msteffen/pachyderm-tools/op"
	"github.com/msteffen/pachyderm-tools/svp/config"

	"github.com/spf13/cobra"
)

const clientNameRegex = "[a-zA-Z0-9_.-]+" // For printing in errors
var /* const */ clientMatcher = regexp.MustCompile(clientNameRegex)

// TODO make this settable
var clientTemplate string = "pachyderm"

// newClient is a Cobra command that creates a new client for working on
// Pachyderm in the pre-configured clients directory, and sets it up to begin
// working
func newClient() *cobra.Command {
	var template string
	newClientCmd := &cobra.Command{
		Use:   "new-client",
		Short: "Create a new client for working on Pachyderm",
		Run: BoundedCommand(1, 1, func(args []string) error {
			clientname := args[0]

			// Validate args
			var (
				templatePath = path.Join(config.Config.ClientDirectory,
					".svp/templates", clientTemplate)
				updateTemplateScript = path.Join(config.Config.ClientDirectory,
					".svp/update-template", clientTemplate)
				initClientScript = path.Join(config.Config.ClientDirectory,
					".svp/init-new-client", clientTemplate)
				clientPath = path.Join(config.Config.ClientDirectory, clientname)
				pachPath   = path.Join(clientPath, "src/github.com/pachyderm/pachyderm")
			)
			if !clientMatcher.MatchString(clientPath) {
				return fmt.Errorf("client name must match %s but was %s", clientNameRegex,
					clientPath)
			}
			if _, err := os.Stat(clientPath); !os.IsNotExist(err) {
				return fmt.Errorf("client %s already exists", clientname)
			}
			if _, err := os.Stat(clientPath); err != nil {
				return fmt.Errorf("could not stat template for %s: %v", clientname)
			}

			// Update template in preparation for creating a new client
			op := op.StartOp()
			op.OutputTo(os.Stdout)
			op.Chdir(templatePath)
			op.Run(updateTemplateScript)
			op.Chdir(config.Config.ClientDirectory)
			op.Run("cp", "-r", "-l", templatePath, clientPath)
			op.Chdir(clientPath)
			op.Run(initClientScript)
			if _, err := os.Stat(pachPath); err == nil {
				op.Chdir(pachPath)
			}
			return op.DetailedError()
		}),
	}
	newClientCmd.Flags().StringVarP(&template, "template", "t", "", "The "+
		"template to use for creating the new client")
	return newClientCmd
}

// ClientCommands returns svp commands related to Pachyderm clients (e.g.
// new-client and delete-client)
func ClientCommands() []*cobra.Command {
	// Add any flags here
	return []*cobra.Command{newClient(), deleteClient}
}
