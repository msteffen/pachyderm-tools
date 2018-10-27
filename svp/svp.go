package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"

	"github.com/msteffen/pachyderm-tools/svp/client"
	"github.com/msteffen/pachyderm-tools/svp/cmds"
	"github.com/msteffen/pachyderm-tools/svp/git"
)

// RootCmd returns the root cobra command (off of which all other svp commands
// branch).
func RootCmd() *cobra.Command {
	// Parse config and initialize config fields
	configpath := path.Join(os.Getenv("HOME"), ".svpconfig")
	if _, err := os.Stat(configpath); os.IsNotExist(err) {
		useDefaultConfig()
	} else {
		configfile, err := os.Open(configpath)
		if err != nil {
			log.Fatalf("could not open config file at %s for reading: %s",
				configpath, err)
		}
		buf := bytes.NewBuffer(nil)
		io.Copy(buf, configfile)
		err = json.Unmarshal(buf.Bytes(), &Config)
		if err != nil {
			log.Fatalf("could not parse ${HOME}/.svpconfig: %s", err.Error())
		}
	}

	// Initialize git information (current branch, etc)
	if err := InitGitInfo(); err != nil {
		fmt.Fprintf(os.Stderr, "could not get info about git repo:\n%s\n(make sure "+
			"this command is being run from inside a git repo)\n", err.Error())
		os.Exit(1)
	}

	// Generate root cobra command & return it
	root := &cobra.Command{
		Use: "svp <command>",
	}
	root.AddCommand(cmds.cmds.DiffCommand())
	root.AddCommand(ChangedFilesCommand())
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
