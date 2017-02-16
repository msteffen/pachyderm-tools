package cmds

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// magicStr is the default value of the --skip flag. This lets us distinguish
// between setting --skip="" (which we interpret as "don't filter") vs not
// setting --skip at all (which we interpret as "use the config- or client-level
// filter")
const magicStr = `d0559e2982835732f88960cc0d87ca25914ff308dcf9247363c7a36537e6be35`

var (
	branch   string // branch to diff against ("master" by default)
	tool     string // tool to view the diff with ("meld" by default)
	upstream bool   // If true, diff against the upstream version of this branch
	skip     string // regex--instruct 'svp diff' to skip files that match
)

var (
	// This error indicates that no commit in --branch contains 'path'
	/* const */ branchFileNotExist = regexp.MustCompile(
		"^fatal: Path '[[:graph:]]+' does not exist in '[[:word:]]+'$")

	// This error indicates that no commit in --branch contains 'path', and also
	// that 'path' is only in the working directory (i.e. no commits in CurBranch)
	/* const */ workingDirOnly = regexp.MustCompile(
		"^fatal: Path '[[:graph:]]+' exists on disk, but not in '[[:word:]]+'.$")
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

// A struct that wraps all of the data structures needed to generate the
// vimscript file used in 'vimdiff()'. It's used to chain together all of the
// io that we do to write out the script, and collect any errors at the end
type vimscriptWriter struct {
	buf *bytes.Buffer // 'buf' will contain the vim script
	out *os.File      // 'out' is the vimscript file -- the eventual target of 'buf'
	err error         // 'err' is where any errors are accumulated
}

// writeString writes 's' to 'w.buf'
func (w *vimscriptWriter) writeString(s string) {
	if w.err != nil {
		return
	}
	if _, err := w.buf.WriteString(s); err != nil {
		w.err = fmt.Errorf("could not write out vimscript command \"%s\":\n%s", s,
			err)
	}
}

// finish writes 'w.buf' to 'w.out' and closes 'w.out'
func (w *vimscriptWriter) finish() {
	if w.err != nil {
		return
	}
	if _, err := w.out.Write(w.buf.Bytes()); err != nil {
		w.err = fmt.Errorf("could not write out vimscript file \"%s\":\n%s",
			w.out.Name(), err)
		return
	}
	if err := w.out.Close(); err != nil {
		w.err = fmt.Errorf("could not close vimscript file \"%s\": %s",
			w.out.Name(), err)
		return
	}
}

// err returns any errors collected by 'w'
func (w *vimscriptWriter) getErr() error {
	return w.err
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
	vimscript, err := ioutil.TempFile(tmpdir, "vim-diffscript")
	defer os.Remove(vimscript.Name()) // delete generated vimscript file when done
	w := vimscriptWriter{
		buf: bytes.NewBuffer(nil),
		out: vimscript,
		err: nil,
	}
	w.writeString("set diffopt=filler,vertical\n")
	w.writeString("edit " + path.Join(GitRoot, files[0]) + "\n")
	w.writeString("diffsplit " + tmpfiles[0].Name() + "\n")
	for i := 1; i < len(files); i++ {
		w.writeString("tabe " + path.Join(GitRoot, files[i]) + "\n")
		w.writeString("diffsplit " + tmpfiles[i].Name() + "\n")
	}
	w.writeString("tabfirst\n")
	w.finish()
	if w.getErr() != nil {
		return w.getErr()
	}

	// Run 'vim' subprocess, with the generated vim script as input
	cmd := exec.Command("vim", "-S", vimscript.Name())
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

// makeDiffTempFile gets the the contents of 'file' in the git branch 'branch'
// and then creates a temporary file F in 'tmpdir' and writes those contents
// into F
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
	cmd := []string{"git", "show", branch + ":" + file}
	cmdString := strings.Join(cmd, " ")
	gitCmd := exec.Command(cmd[0], cmd[1:]...)
	stdoutPipe, err := gitCmd.StdoutPipe()
	errMsg := bytes.NewBuffer(nil) // Error message from 'git show' will go here
	stderrPipe, err := gitCmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("could not get stdout pipe from \"%s\": %s",
			cmdString, err)
	}
	err = gitCmd.Start()
	if err != nil {
		return nil, fmt.Errorf("could not start command \"%s\": %s", cmdString, err)
	}
	_, err = io.Copy(tmpfile, stdoutPipe)
	if err != nil {
		return nil, fmt.Errorf("could not write contents of \"%s\" in 'master' "+
			"to '%s': %s", file, tmpfile.Name(), err)
	}
	_, err = io.Copy(errMsg, stderrPipe)
	if err != nil {
		return nil, fmt.Errorf("could not copy error message from \"%s\": %s",
			cmdString, err)
	}
	if err = gitCmd.Wait(); err != nil {
		trimmedErr := bytes.TrimSpace(errMsg.Bytes())
		// fmt.Printf("Received message: \"%s\"\n", trimmedErr)
		// fmt.Printf("workingDirOnly.Match(errMsg.Bytes()): %t\n", workingDirOnly.Match(trimmedErr))
		if branchFileNotExist.Match(trimmedErr) ||
			workingDirOnly.Match(trimmedErr) {
			// fmt.Println("returning empty tmpfile")
			return tmpfile, nil // No error, but 'file' does not exist in 'branch'
		}
		return nil, fmt.Errorf("command \"%s\" did not run successfully: %s",
			cmdString, err)
	}
	return tmpfile, nil
}

// diff is a cobra command that applies the diff tool to a given file, or to
// all of the files changed in this workspace
var diff = &cobra.Command{
	Use:   "diff <filename>",
	Short: "Diff files against some other branch of the pachyderm repo",
	Run: unboundedCommand(func(args []string) error {
		// Sanitize 'branch' and don't run diff if 'branch' doesn't make sense
		if CurBranch == "master" && !upstream {
			return fmt.Errorf("current branch is 'master'...cannot diff master " +
				"against itself")
		} else if branch != "master" && upstream {
			return fmt.Errorf("cannot set both --branch and --upstream")
		}
		if upstream {
			branch = "origin/" + CurBranch
		}

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
			files0, err = ModifiedFiles(CurBranch, branch)
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
		} else {
			return fmt.Errorf("did not recognize diff command %s; must be \"vim\" " +
				"or \"meld\"")
		}
	}),
}

// DiffCommand returns a collection of Cobra commands related to diffing files
// that have been modified in a Pachyderm client
func DiffCommand() *cobra.Command {
	diff.PersistentFlags().StringVarP(&branch, "branch", "b", "master",
		"The branch to diff against")
	diff.PersistentFlags().StringVarP(&tool, "tool", "t", "meld",
		"The branch to diff against")
	diff.PersistentFlags().BoolVar(&upstream, "upstream", false,
		"If true, diff this branch against the upstream version of itself. At "+
			"most one of --upstream and --branch should be set")
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
