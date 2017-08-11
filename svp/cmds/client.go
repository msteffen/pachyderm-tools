package cmds

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/sync/errgroup"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/msteffen/pachyderm-tools/op"
	"github.com/spf13/cobra"
)

const clientNameRegex = "[a-zA-Z0-9_.-]+" // For printing in errors
var /* const */ clientMatcher = regexp.MustCompile(clientNameRegex)

// dircreator is basically an error accumulator for making directories. You can
// call dircreator.mkdir() over and over, and only check errors at the end
type dircreator struct {
	err error
}

// replaceLine replaces any occurances of the text 'needle' in the file
// 'filePath' with the text in 'replace'. Note that 'needle' must not cross line
// boundaries
func replaceLine(filePath, needle, replace string) error {
	// stat 'filePath'
	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("could not stat '%s': %s", filePath, err)
	}
	out := bytes.NewBuffer(make([]byte, 0, stat.Size()))

	// if 'filePath' exists, open it
	file, err := os.OpenFile(filePath, os.O_RDWR, 0664)
	if err != nil {
		return fmt.Errorf("could not open '%s': %s", filePath, err)
	}

	// Scan the lines of 'file' and replace any matches of 'needle' with 'replace'
	// (write the new contents to 'out')
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, needle); idx > 0 {
			out.WriteString(line[:idx])
			out.WriteString(replace)
			out.WriteString(line[idx+len(needle):])
		} else {
			out.Write(scanner.Bytes())
		}
		out.WriteRune('\n')
	}

	// write 'out' into file and close it.
	if _, err := file.WriteAt(out.Bytes(), 0); err != nil {
		return fmt.Errorf("could not overwrite '%s': %s", filePath, err)
	}
	if err := file.Truncate(int64(out.Len())); err != nil {
		return fmt.Errorf("could not truncate '%s': %s", filePath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("could not close '%s': %s", filePath, err)
	}
	return nil
}

// addLine appends 'line' to the file at  'filePath'.
func addLine(filePath, line string) error {
	// stat 'filePath'
	_, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("could not stat '%s': %s", filePath, err)
	}
	// open 'filepath' for writing, and begin writing at the end. If 'filepath'
	// does not exist, create it.
	// See http://man7.org/linux/man-pages/man2/openat.2.html for more
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)
	if err != nil {
		return fmt.Errorf("could not open '%s': %s", filePath, err)
	}

	// write 'out' into file and close it.
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("could not append to '%s': %s", filePath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("could not close '%s': %s", filePath, err)
	}
	return nil
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
		if !clientMatcher.MatchString(clientpath) {
			return fmt.Errorf("client name must match %s but was %s", clientNameRegex,
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

			// Create a git branch matching the clientname, and delete "master"
			op.Run("git", "checkout", "-b", clientname)
			op.Run("git", "branch", "-d", "master")
			if op.LastError() != nil {
				return fmt.Errorf("couldn't create client branch:\n%s", op.DetailedError())
			}

			// Update .git/config so that the 'origin' remote repo uses ssh instead
			// of http
			fmt.Println("Updating .git/config...")
			if err := replaceLine("./.git/config",
				"url = https://github.com/pachyderm/pachyderm",
				"url = git@github.com:pachyderm/pachyderm.git"); err != nil {
				return fmt.Errorf("could not update .git/config: %s", err)
			}
			if err := replaceLine("Dockerfile",
				"https://get.docker.com/builds/Linux/x86_64/docker-1.12.1.tgz",
				"https://get.docker.com/builds/Linux/x86_64/docker-1.11.1.tgz"); err != nil {
				return fmt.Errorf("could not update Dockerfile: %s", err)
			}
			// Add known differences to gitignore and agignore
			gitIgnoreAdditions := "src/server/pachyderm_test.go.old\nDockerfile"
			if err := addLine("./.gitignore", gitIgnoreAdditions); err != nil {
				return err
			}
			fmt.Printf("Adding to .gitignore:\n%s\n", gitIgnoreAdditions)

			agIgnoreAdditions := "vendor"
			if err := addLine("./.agignore", agIgnoreAdditions); err != nil {
				return err
			}
			fmt.Printf("Adding to .agignore:\n%s\n", agIgnoreAdditions)
			return nil
		})

		// Return once both operations are finished
		return eg.Wait()
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
