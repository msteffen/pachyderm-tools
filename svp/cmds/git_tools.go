package cmds

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/msteffen/pachyderm-tools/svp/config"
	"github.com/msteffen/pachyderm-tools/svp/git"

	"github.com/spf13/cobra"
)

var branch string // branch to diff against for 'diff' and 'changed' ("master" by default)

func checkGitRepoAndCdToRoot() error {
	if git.Root == "" {
		return fmt.Errorf("this command must be run from inside a git repo")
	}
	if err := os.Chdir(git.Root); err != nil {
		return fmt.Errorf("could not cd to %s: %v", git.Root, err)
	}
	return nil
}

// gitBoundedCommand is like BoundedCommand for commands that interact with
// git.  In addition to the BoundedCommand checks, this also checks that the
// command is being run from inside a git repo and changes directory to the
// root of the git repo
func gitBoundedCommand(minargs, maxargs int, f Command) func(*cobra.Command, []string) {
	return BoundedCommand(minargs, maxargs, func(args []string) error {
		if err := checkGitRepoAndCdToRoot(); err != nil {
			return err
		}
		return f(args)
	})
}

// gitUnboundedCommand is like UnboundedCommand for commands that interact with
// git.  In addition to the BoundedCommand checks, this also checks that the
// command is being run from inside a git repo and changes directory to the
// root of the git repo
func gitUnboundedCommand(f Command) func(*cobra.Command, []string) {
	return UnboundedCommand(func(args []string) error {
		if err := checkGitRepoAndCdToRoot(); err != nil {
			return err
		}
		return f(args)
	})
}

// changedFilesCommand returns a Cobra command that prints the output of
// modifiedFiles()
func changedFilesCommand() *cobra.Command {
	changed := &cobra.Command{
		Use:   "changed",
		Short: "List the files that have changed between this branch and master",
		Run: gitBoundedCommand(0, 0, func(args []string) error {
			if git.Root == "" {
				return fmt.Errorf("changed must be run from inside a git repo")
			}
			// Sanitize 'branch' and don't run diff if 'branch' doesn't make sense
			files, err := modifiedFiles(git.CurBranch, branch)
			if err != nil {
				return err
			}
			fmt.Println(strings.Join(files, "\n"))
			return nil
		}),
	}

	// 'branch' is declared in diff.go
	changed.PersistentFlags().StringVarP(&branch, "branch", "b", "origin/master",
		"Show changed files relative to this branch")
	return changed
}

// diffCommand returns a cobra command that applies the diff tool to a given
// file, or to all of the files changed in this workspace
func diffCommand() *cobra.Command {
	var tool string // tool to view the diff with ("meld" by default)
	var skip string // regex--instruct 'svp diff' to skip files that match
	diff := &cobra.Command{
		Use:   "diff <filename>",
		Short: "Diff files against some other branch of the pachyderm repo",
		Run: gitUnboundedCommand(func(args []string) error {
			// Compile regex for skipping uninteresting files
			skip2 := config.Config.Diff.Skip
			if skip != magicStr {
				skip2 = skip
			}
			skipRe, err := regexp.Compile(config.Config.Diff.Skip)
			if err != nil {
				return fmt.Errorf("could not compile regex \"%s\" for skipping files: %s",
					skip2, err)
			}

			// Get either 1) list of files that have changed between 'master' and
			// current branch, or 2) files passed via args.
			var files []string
			if len(args) == 0 {
				files0, err := modifiedFiles(git.CurBranch, branch)
				if err != nil {
					return fmt.Errorf("could not get list of changed files "+
						"(to diff):\n%s", err)
				}
				// Filter out uninteresting files
				for _, file := range files0 {
					if !skipRe.MatchString(file) {
						files = append(files, file)
					}
				}
			} else {
				for _, arg := range args {
					fullFilename := path.Join(git.Root, arg)
					if _, err := os.Stat(fullFilename); os.IsNotExist(err) {
						return fmt.Errorf("file \"%s\" does not exist", fullFilename)
					}
				}
				files = args
			}
			if len(files) == 0 {
				return fmt.Errorf("no differing files found between \"%s\" and \"%s\"",
					git.CurBranch, branch)
			}
			sort.Strings(files)

			// Create a temporary directory to contain copies of 'files' that will be
			// diffed against (i.e. the contents of 'files' in 'branch').
			tmpdir, err := ioutil.TempDir("/tmp", "svp-diff-master-files-")
			if err != nil {
				return fmt.Errorf("Could not create temporary file: %s", err)
			}
			defer os.RemoveAll(tmpdir)

			// Populate the temporary directory with tmp files containing file
			// contents from 'branch'
			tmpfiles := make([]*os.File, len(files))
			for i, file := range files {
				tmpfiles[i], err = makeDiffTempFile(branch, tmpdir, file)
				if err != nil {
					return err
				}
			}

			// Run diff tool selected by user
			if fn, ok := diffFn[tool]; ok {
				err := fn(tmpdir, files, tmpfiles)
				if err != nil {
					return fmt.Errorf("could not run diff tool %s: %s", tool, err)
				}
				return nil
			}
			return fmt.Errorf("did not recognize diff command %s; must be \"vim\" "+
				"or \"meld\"", tool)
		}),
	}

	diff.PersistentFlags().StringVarP(&branch, "branch", "b", "origin/master",
		"The branch to diff against")
	diff.PersistentFlags().StringVarP(&tool, "tool", "t", "meld",
		"The branch to diff against")
	diff.PersistentFlags().StringVar(&skip, "skip", magicStr,
		"A regex that is used to skip files encountered by 'svp diff' (e.g. "+
			"vendored files or .gitignore)")
	return diff
}

// GitHelperCommands returns Cobra commands that print the outputs of
// CurBranch() and GitRoot()
func GitHelperCommands() []*cobra.Command {
	return []*cobra.Command{
		changedFilesCommand(),
		diffCommand(),
	}
}
