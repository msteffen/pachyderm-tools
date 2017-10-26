package cmds

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/msteffen/pachyderm-tools/op"
	"github.com/spf13/cobra"
)

// magicStr is the default value of the --skip flag. This lets us distinguish
// between setting --skip="" (which we interpret as "don't filter") vs not
// setting --skip at all (which we interpret as "use the config- or client-level
// filter")
const magicStr = `d0559e2982835732f88960cc0d87ca25914ff308dcf9247363c7a36537e6be35`

var (
	branch string // branch to diff against ("master" by default)
	tool   string // tool to view the diff with ("meld" by default)
	skip   string // regex--instruct 'svp diff' to skip files that match
)

var (
	// This error indicates that no commit in --branch contains 'path'
	/* const */ branchFileNotExist = regexp.MustCompile(
		"^fatal: Path '[[:graph:]]+' does not exist in '[[:word:]/]+'$")

	// 'git show' emits this error if no commit in --branch contains 'path', but
	// 'path' is in the working directory
	/* const */
	workingDirOnly = regexp.MustCompile(
		"^fatal: Path '[[:graph:]]+' exists on disk, but not in '[[:word:]/]+'.$")
)

// meld shows the user the diff between 'files' and 'tmpfiles' with meld
func meld(tmpdir string, files []string, tmpfiles []*os.File) error {
	// create one tab per file, and put the peer file (i.e. the file from
	// "master") on the left and the client file on the right
	cmd := make([]string, 3*len(files))
	for i := 0; i < len(files); i++ {
		cmd[3*i] = "--diff"
		cmd[(3*i)+1] = tmpfiles[i].Name()
		cmd[(3*i)+2] = path.Join(GitRoot, files[i])
	}
	output, err := exec.Command("meld", cmd...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("(%v) %s", err, output)
	}
	return nil
}

// writeToTmpfile write the data in 'contents' to a new tempfile that it creates
// with the prefix  'prefix' in the directory 'dir'. The file is closed, and the
// name of the new file is returned (if it was created, along with any errors)
func writeToTmpfile(dir, prefix string, contents []byte) (string, error) {
	// Create the vimscript tempfile
	vimscript, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		return "", fmt.Errorf("could not create vimscript tempfile: %s", err)
	}
	name := vimscript.Name() // 'vimscript' now exists on disk; must return 'name'

	// Write contents to 'vimscript'
	_, err = vimscript.Write(contents)
	if err != nil {
		return name, fmt.Errorf("could not write out vimscript: %s", err)
	}

	// close 'vimscript
	err = vimscript.Close()
	if err != nil {
		return name, fmt.Errorf("could not close vimscript: %s", err)
	}
	return name, nil
}

// vimdiff shows the user the diff between 'files' and 'tmpfiles' with vimdiff
func vimdiff(tmpdir string, files []string, tmpfiles []*os.File) error {
	if len(files) == 0 || len(tmpfiles) == 0 {
		return nil
	}
	if len(files) != len(tmpfiles) {
		return fmt.Errorf("could not compare %v to %v; different lengths", files,
			tmpfiles)
	}

	// Generate a vim script to open all of the modified files in this branch
	// as vim tabs
	buf := bytes.Buffer{} // bytes.Buffer.Write() does not return errors
	buf.WriteString(fmt.Sprintf("set diffopt=filler,vertical\n"))
	buf.WriteString(fmt.Sprintf("edit %s\n", path.Join(GitRoot, files[0])))
	buf.WriteString(fmt.Sprintf("diffsplit %s\n", tmpfiles[0].Name()))
	for i := 1; i < len(files); i++ {
		buf.WriteString(fmt.Sprintf("tabe %s\n", path.Join(GitRoot, files[i])))
		buf.WriteString(fmt.Sprintf("diffsplit %s\n", tmpfiles[i].Name()))
	}
	buf.WriteString("tabfirst\n")
	name, err := writeToTmpfile(tmpdir, "vim-diffscript", buf.Bytes())
	if len(name) > 0 {
		defer os.Remove(name) // delete vimscript file (even if err != nil)
	}
	if err != nil {
		return err
	}

	// Run 'vim' subprocess, with the generated vim script as input
	cmd := exec.Command("vim", "-S", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// open /dev/tty for vim; see [note] at the end
	if tty, err := os.Open("/dev/tty"); err == nil {
		cmd.Stdin = tty
	} else {
		fmt.Fprintf(os.Stderr, "could not use /dev/tty as vim input (you may have "+
			"to run 'reset' afterwards: %s\n", err)
	}
	err = cmd.Run()
	return err
}

var diffFn = map[string]func(string, []string, []*os.File) error{
	"meld": meld,
	"vim":  vimdiff,
}

// makeDiffTempFile:
// 1) gets the the contents of 'file' in the git branch 'branch'
// 2) creates a temporary file 'tmpdir'
// 3) write the data from (1) into file from (2)
func makeDiffTempFile(branch, tmpdir, file string) (*os.File, error) {
	// Create a temporary file
	tmpfile, err := ioutil.TempFile(tmpdir, strings.Replace(file,
		"/", "_", -1))
	if err != nil {
		return nil, fmt.Errorf("Could not create temporary file for \"%s\":\n%s",
			file, err)
	}
	defer tmpfile.Close()

	// cat contents of read file in 'master' to tmp file
	op := op.StartOp()
	op.OutputTo(tmpfile)
	op.Run("git", "show", branch+":"+file)
	if op.LastError() != nil {
		if branchFileNotExist.Match(op.LastErrorMsg()) ||
			workingDirOnly.Match(op.LastErrorMsg()) {
			return tmpfile, nil // No error, but 'file' does not exist in 'branch'
		}
		return nil, op.DetailedError()
	}
	return tmpfile, nil
}

// diff is a cobra command that applies the diff tool to a given file, or to
// all of the files changed in this workspace
var diff = &cobra.Command{
	Use:   "diff <filename>",
	Short: "Diff files against some other branch of the pachyderm repo",
	Run: unboundedCommand(func(args []string) error {
		// Compile regex for skipping uninteresting files
		skip2 := Config.DiffSkip
		if skip != magicStr {
			skip2 = skip
		}
		skipRe, err := regexp.Compile(Config.DiffSkip)
		if err != nil {
			return fmt.Errorf("could not compile regex \"%s\" for skipping files: %s",
				skip2, err)
		}

		// Get either 1) list of files that have changed between 'master' and
		// current branch, or 2) files passed via args.
		var files []string
		if len(args) == 0 {
			files0, err := ModifiedFiles(CurBranch, branch)
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
				fullFilename := path.Join(GitRoot, arg)
				if _, err := os.Stat(fullFilename); os.IsNotExist(err) {
					return fmt.Errorf("file \"%s\" does not exist", fullFilename)
				}
			}
			files = args
		}
		if len(files) == 0 {
			return fmt.Errorf("no differing files found between \"%s\" and \"%s\"",
				CurBranch, branch)
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
		return fmt.Errorf("did not recognize diff command %s; must be \"vim\" " +
			"or \"meld\"")
	}),
}

// DiffCommand returns a collection of Cobra commands related to diffing files
// that have been modified in a Pachyderm client
func DiffCommand() *cobra.Command {
	diff.PersistentFlags().StringVarP(&branch, "branch", "b", "master",
		"The branch to diff against")
	diff.PersistentFlags().StringVarP(&tool, "tool", "t", "meld",
		"The branch to diff against")
	diff.PersistentFlags().StringVar(&skip, "skip", magicStr,
		"A regex that is used to skip files encountered by 'svp diff' (e.g. "+
			"vendored files or .gitignore)")
	return diff
}

// [note] on running vim as a subprocess
// ---
// If you don't set vim's input to /dev/tty directly, it fails to reset bash
// codes that it should, such as -echo. See:
// http://askubuntu.com/questions/171449/shell-does-not-show-typed-in-commands-reset-works-but-what-happened
// and
// https://superuser.com/questions/336016/invoking-vi-through-find-xargs-breaks-my-terminal-why
