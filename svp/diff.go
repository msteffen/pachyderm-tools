package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/spf13/cobra"
)

// Diff applies the diff tool to a given file, or to all of the files changed in this
// workspace
var (
	repo string // repo to diff against ("master" by default)

	diff *cobra.Command = &cobra.Command{
		Use:   "diff <filename>",
		Short: "Diff a particular file with the 'master' branch of the pachyderm repo",
		Run: boundedCommand(0, 1, func(args []string) error {
			gitRoot, err := GitRoot()
			if err != nil {
				return fmt.Errorf("Could not get root of current git repo: %s", err)
			}

			// Get list of files that have changed between 'master' and current branch
			var files []string
			if len(args) == 0 {
				var err error
				files, err = ModifiedFiles()
				if err != nil {
					return fmt.Errorf("Could not get list of changed files "+
						"(to diff):\n%s", err)
				}
			} else {
				for _, arg := range args {
					fullFilename := path.Join(gitRoot, arg)
					if _, err := os.Stat(fullFilename); os.IsNotExist(err) {
						return fmt.Errorf("File \"%s\" does not exist", fullFilename)
					}
				}
				files = args
			}

			// Create a temporary directory to contain copies of 'files' that will be
			// diffed against (i.e. the contents of those files in the 'master'
			// branch)
			tmpdir, err := ioutil.TempDir("/tmp", "svp-diff-master-files-")
			if err != nil {
				return fmt.Errorf("Could not create temporary file: %s", err)
			}
			defer os.RemoveAll(tmpdir)

			// Populate the temporary directory with tmp files containing file
			// contents from 'master'
			tmpfiles := make([]*os.File, len(files))
			for i, file := range files {
				// Create a temporary file
				tmpfiles[i], err = ioutil.TempFile(tmpdir, strings.Replace(file,
					"/", "_", -1))
				if err != nil {
					return fmt.Errorf("Could not create temporary file for \"%s\":\n%s",
						file, err)
				}

				// cat contents of read file in 'master' to tmp file
				cmdString := "git show master:" + file // for errors
				gitCmd := exec.Command("git", "show", "master:"+file)
				stdoutPipe, err := gitCmd.StdoutPipe()
				if err != nil {
					return fmt.Errorf("Could not get stdout pipe from \"%s\": %s", cmdString, err)
				}
				err = gitCmd.Start()
				if err != nil {
					return fmt.Errorf("Could not start command \"%s\": %s", cmdString, err)
				}
				_, err = io.Copy(tmpfiles[i], stdoutPipe)
				if err != nil {
					return fmt.Errorf("Could not write contents of \"%s\" in 'master' "+
						"to tmpfiles[%d]: %s", file, i, err)
				}
				if err = gitCmd.Wait(); err != nil {
					return fmt.Errorf("Command \"%s\" did not run successfully: %s", cmdString, err)
				}
				tmpfiles[i].Close()
			}

			// Run meld on tmp file and real file. Do this by creating one tab per
			// file, and putting the 'master' copy on the left and branch copy on
			// the right
			meldFlags := make([]string, 3*len(files))
			for i := 0; i < len(files); i++ {
				meldFlags[3*i] = "--diff"
				meldFlags[(3*i)+1] = tmpfiles[i].Name()
				meldFlags[(3*i)+2] = path.Join(gitRoot, files[i])
			}
			_, err = exec.Command("meld", meldFlags...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("Could not start meld: %s", err)
			}
			return nil
		}),
	}
)

func DiffCommand() *cobra.Command {
	diff.PersistentFlags().StringVar(&repo, "repo", "master", "The repo to diff against")
	return diff
}
