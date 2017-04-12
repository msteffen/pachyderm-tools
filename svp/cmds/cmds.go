package cmds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"github.com/spf13/cobra"
)

// Config is a struct containing all fields defined in the .svpconfig file
// (this is how configured values can be accessed)
var Config struct {
	ClientDirectory string // The top-level directory containing all clients
	DiffTool        string // The user's preferred tool for diffing branches
	DiffSkip        string // Regex to let users skip certain files in svp diff
	// TODO: DiffSkip should be settable per client (with maybe a global default?)
	// (maybe a flag override allowed too?)
}

func useDefaultConfig() {
	Config.ClientDirectory = path.Join(os.Getenv("HOME"), "clients")
	Config.DiffTool = "meld"
}

type command func([]string) error

func unboundedCommand(f command) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := f(args); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	}
}

// Bounded command is a convenience function that takes a lower and upper bound
// on the number of positional arguments that a cobra command can recieve, and
// a definition of the command itself (in 'f') and return a func that can be
// added to a Cobra command-line tool
// TODO print usage
func boundedCommand(minargs, maxargs int, f command) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		var err error
		argc := len(args)
		switch {
		case minargs > maxargs:
			err = fmt.Errorf("invalid arguments to 'boundedCommand': 'minargs' "+
				"must be <= 'maxargs', but got %d > %d", minargs, maxargs)
		case minargs == maxargs && argc != minargs:
			err = fmt.Errorf(fmt.Sprintf("expected exactly %d arguments, but got %d",
				minargs, argc))
		case argc < minargs:
			err = fmt.Errorf("expected at least %d arguments, but got %d",
				minargs, argc)
		case argc > maxargs:
			err = fmt.Errorf("expected at most %d arguments, but got %d",
				maxargs, argc)
		default:
			err = f(args)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	}
}

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
	root.AddCommand(DiffCommand())
	root.AddCommand(ChangedFilesCommand())
	for _, cmd := range GitHelperCommands() {
		root.AddCommand(cmd)
	}
	for _, cmd := range ClientCommands() {
		root.AddCommand(cmd)
	}
	return root
}
