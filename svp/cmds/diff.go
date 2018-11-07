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
	"github.com/msteffen/pachyderm-tools/svp/config"
	"github.com/msteffen/pachyderm-tools/svp/git"

	"github.com/spf13/cobra"
)

// magicStr is the default value of the --skip flag. This lets us distinguish
// between setting --skip="" (which we interpret as "don't filter") vs not
// setting --skip at all (which we interpret as "use the config- or client-level
// filter")
const magicStr = `d0559e2982835732f88960cc0d87ca25914ff308dcf9247363c7a36537e6be35`

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
		cmd[(3*i)+2] = path.Join(git.Root, files[i])
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
	// Create the tempfile
	tempfile, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		return "", fmt.Errorf("could not create tempfile %sSUFFIX: %s", prefix, err)
	}
	name := tempfile.Name() // 'tempfile' now exists on disk; must return 'name'

	// Write contents to 'tempfile'
	_, err = tempfile.Write(contents)
	if err != nil {
		return name, fmt.Errorf("could not write out tempfile %q: %s", name, err)
	}

	// close 'tempfile
	err = tempfile.Close()
	if err != nil {
		return name, fmt.Errorf("could not close tempfile %q: %s", name, err)
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
	buf.WriteString(fmt.Sprintf("edit %s\n", path.Join(git.Root, files[0])))
	buf.WriteString(fmt.Sprintf("diffsplit %s\n", tmpfiles[0].Name()))
	for i := 1; i < len(files); i++ {
		buf.WriteString(fmt.Sprintf("tabe %s\n", path.Join(git.Root, files[i])))
		buf.WriteString(fmt.Sprintf("diffsplit %s\n", tmpfiles[i].Name()))
	}
	buf.WriteString("tabfirst\n")
	name, err := writeToTmpfile(tmpdir, "vim-diffscript-", buf.Bytes())
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
