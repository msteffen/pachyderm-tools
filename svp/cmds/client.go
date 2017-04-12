package cmds

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/sync/errgroup"
	"os"
	"path"
	"regexp"

	"github.com/msteffen/pachyderm-tools/op"
	"github.com/spf13/cobra"
)

const fileRegex = "[a-zA-Z0-9_.-]+" // For printing in errors
var /* const */ fileMatcher = regexp.MustCompile(fileRegex)

// dircreator is basically an error accumulator for making directories. You can
// call dircreator.mkdir() over and over, and only check errors at the end
type dircreator struct {
	err error
}

// mkdir creates a directory, as part of a chain of such calls. If a previous
// call to mkdir has failed, this is a no-op.
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
	Use:   "new-client",
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

		// Download data (pachyderm repo and vim binaries) into the new client
		// TODO: This should be copied instead of downloaded every time. This takes
		// several seconds to finish
		var eg errgroup.Group
		os.Setenv("GOPATH", clientpath)
		os.Chdir(clientpath)
		// Install vim-go binaries in a separate goroutine (slow)
		eg.Go(func() error {
			fmt.Println("Beginning to install vim-go binaries...")
			op := op.StartOp()
			op.OutputTo(os.Stdout)
			op.Run("vim", "-c", ":GoUpdateBinaries", "-c", ":qa")
			if op.LastError() != nil {
				return fmt.Errorf("couldn't install go binaries:\n%s", op.DetailedError())
			}
			fmt.Println("vim-go binaries successfully installed")
			return nil
			return nil
		})

		// Get the pachyderm repo (slow) & update .git/config in separate goroutine
		eg.Go(func() error {
			fmt.Println("Beginning to fetch Pachyderm repo...")
			op := op.StartOp()
			op.Run("go", "get", "github.com/pachyderm/pachyderm")
			fmt.Println("pachyderm repo fetched")
			os.Chdir(path.Join(clientpath, "src/github.com/pachyderm/pachyderm"))

			// Create a git branch matching the clientname
			// TODO(msteffen): let the user specify the branch, in case you want to
			// have multiple clients for the same non-master branch
			op.Run("git", "checkout", "-b", clientname)
			if op.LastError() != nil {
				return fmt.Errorf("couldn't create client branch:\n%s", op.DetailedError())
			}

			// Update .git/config so that the 'origin' remote repo uses ssh instead
			// of http
			fmt.Println("Updating .git/config...")
			stat, err := os.Stat("./.git/config")
			if err != nil {
				return fmt.Errorf("could not stat .git/config to update origin: %s", err)
			}
			gitconf, err := os.OpenFile("./.git/config", os.O_RDWR, 0664)
			if err != nil {
				return fmt.Errorf("could not open .git/config to update origin: %s", err)
			}
			// (we're replacing one line of config with another that's the same
			// length)
			out := bytes.NewBuffer(make([]byte, 0, stat.Size()))
			scanner := bufio.NewScanner(gitconf)
			for scanner.Scan() {
				if scanner.Text() == "\turl = https://github.com/pachyderm/pachyderm" {
					out.Write([]byte("\turl = git@github.com:pachyderm/pachyderm.git"))
				} else {
					out.Write(scanner.Bytes())
				}
				out.WriteByte('\n')
			}
			// Replace gitconf with new contents
			if _, err := gitconf.WriteAt(out.Bytes(), 0); err != nil {
				return fmt.Errorf("could not overwrite .git/config: %s", err)
			}
			if err := gitconf.Truncate(int64(out.Len())); err != nil {
				return fmt.Errorf("could not truncate .git/config: %s", err)
			}
			if err := gitconf.Close(); err != nil {
				return fmt.Errorf("could not close .git/config: %s", err)
			}
			return nil
		})

		// Return once both operations are finished
		eg.Wait()
		return nil
	}),
}

// deleteClient is a Cobra command that deletes an existing pachyderm client
var deleteClient = &cobra.Command{
	Use:   "delete-client",
	Short: "Delete a Pachyderm client",
	Run: boundedCommand(1, 1, func(args []string) error {
		clientname := args[0]

		// Check if the client path exists (exit early if not)
		clientpath := path.Join(Config.ClientDirectory, clientname)
		if _, err := os.Stat(clientpath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("could not delete %s: client does not exist",
					clientname)
			}
			return fmt.Errorf("could not stat %s: %s", clientpath, err)
		}

		// Check if the pachyderm repo exists (exit early if not -- maybe delete the
		// client anyway). If so, move to that directory.
		gitpath := path.Join(clientpath, "src/github.com/pachyderm/pachyderm")
		if _, err := os.Stat(gitpath); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("%s does not exist. Delete client dir anyway? y/N:")
				stdinScanner := bufio.NewScanner(os.Stdin)
				stdinScanner.Scan()
				if char := stdinScanner.Text()[0]; char == 'y' || char == 'Y' {
					return os.RemoveAll(clientpath)
				}
			}
			return fmt.Errorf("could not stat %s: %s", gitpath, err)
		}
		if err := os.Chdir(gitpath); err != nil {
			return err
		}

		// Delete the client branch, and then delete the whole client directory
		delOp := op.StartOp()
		cmds := [][]string{
			// The first set of commands pull master, so that if you've already merged
			// a PR with your changes, 'git branch -d' won't complain about unmerged
			// commits.
			// TODO(msteffen): You should be able to specify a custom base branch.
			{"git", "stash"},
			{"git", "checkout", "master"},
			{"git", "pull", "origin", "master"},
		}
		for _, cmd := range cmds {
			delOp.Run(cmd...)
			if delOp.LastError() != nil {
				return fmt.Errorf("could not execute '%s':\n%s", cmd,
					delOp.DetailedError())
			}
		}
		if delOp.Run("git", "branch", "-d", clientname); delOp.LastError() != nil {
			return fmt.Errorf("could not delete branch: %s", delOp.DetailedError())
		}
		if err := os.Chdir(Config.ClientDirectory); err != nil {
			return err
		}
		if err := os.RemoveAll(clientpath); err != nil {
			return fmt.Errorf("could not remove client directory (%s): %s",
				clientpath, err)
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

// ClientCommands returns svp commands related to Pachyderm clients (e.g.
// new-client and delete-client)
func ClientCommands() []*cobra.Command {
	// Add any flags here
	return []*cobra.Command{newClient, deleteClient}
}
