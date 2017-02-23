package cmds

import (
	"fmt"
	"os"
	"path"
	"regexp"

	"github.com/spf13/cobra"
)

const fileRegex = "[a-zA-Z0-9_.-]+" // For printing in errors
var /* const */ fileMatcher = regexp.MustCompile(fileRegex)

// dircreator is basically an error accumulator for making directories. You can
// call dircreator.mkdir() over and over, and only check errors at the end
type dircreator struct {
	err error
}

// Creates a directory, as part of a chain of such calls. If a previous call to
// mkdir has failed, this is a no-op.
func (f *dircreator) mkdir(path string, mode os.FileMode, desc string) {
	if f.err != nil {
		return
	}
	f.err = os.Mkdir(path, mode)
	if f.err != nil {
		if os.IsExist(f.err) {
			f.err = nil // not an error, just continue
			return
		}
		f.err = fmt.Errorf("%s at %s does not exist and could not be created: %s",
			desc, path, f.err.Error())
	}
}

// newClient is a Cobra command that creates a new client for working on
// Pachyderm in the pre-configured clients directory, and sets it up to begin
// working
var newClient = &cobra.Command{
	Use:   "newclient",
	Short: "Create a new client for working on Pachyderm",
	Run: boundedCommand(1, 1, func(args []string) error {
		clientname := args[0]

		// Create the directory tree of a new client (i.e.
		// /clients/${client}/{src,pkg,dir})
		f := dircreator{}
		f.mkdir(Config.ClientDirectory, 077, "parent directory of clients")
		clientpath := path.Join(Config.ClientDirectory, clientname)
		if !fileMatcher.MatchString(clientpath) {
			return fmt.Errorf("client name must match %s but was %s", fileRegex,
				clientpath)
		}
		if _, err := os.Stat(clientpath); !os.IsNotExist(err) {
			return fmt.Errorf("client %s already exists", clientname)
		}
		f.mkdir(clientpath, 0777, "client directory")
		f.mkdir(path.Join(clientpath, "src"), 0777, "\"src\" under client")
		f.mkdir(path.Join(clientpath, "bin"), 0777, "\"bin\" under client")
		f.mkdir(path.Join(clientpath, "pkg"), 0777, "\"pkg\" under client")
		if f.err != nil {
			return f.err
		}

		// Pulls the pachyderm repo into the client directory
		os.Setenv("GOPATH", clientpath)
		os.Chdir(clientpath)
		fmt.Println("Fetching Pachyderm repo...")
		op := StartOp()
		op.Run("go", "get", "github.com/pachyderm/pachyderm")
		os.Chdir(path.Join(clientpath, "src/github.com/pachyderm/pachyderm"))
		op.Run("git", "checkout", "-b", clientname)
		if op.LastError() != nil {
			return fmt.Errorf("couldn't create client branch:\n%s", op.DetailedError())
		}
		return nil
	}),
}

var sync = &cobra.Command{
	Use:   "sync",
	Short: "update this client (sync master, and rebase working branch on top of it",
	Run: boundedCommand(0, 0, func(args []string) error {
		return nil
	}),
}

// NewClientCommand returns a Cobra command that creates a new Pachyderm client
func NewClientCommand() *cobra.Command {
	// Add any flags here
	return newClient
}
