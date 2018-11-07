package cmds

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/msteffen/pachyderm-tools/svp/git"
)

// Regexes for parsing the output of 'git status' in uncommittedFiles()
var (
	/* const */ whitespace = regexp.MustCompile("^[ \t\r\n]*$")
	/* const */ statusPlain = regexp.MustCompile("^(..) ([^ ]*)$")
	/* const */ statusArrow = regexp.MustCompile("^(..) [^ ]* -> ([^ ]*)$")

	// alwaysModified contains files that the svp tool modifies when creating a new
	// client, along with files that aren't edited by hand (e.g. the 'pachd'
	// binary), so 'svp changed' should ignore them
	/* const */
	alwaysModified = map[string]struct{}{
		".gitignore": struct{}{},
		"Dockerfile": struct{}{},
		".ignore":    struct{}{},
		"pachd":      struct{}{},
	}
)

// uncommittedFiles returns the list of files that have changed in the working
// tree, by parsing the output of 'git status'.
//
// All files are relative to the root of the current git repo. Used by
// modifiedFiles()
func uncommittedFiles() (map[string]struct{}, error) {
	cmdString := "git status --porcelain"
	statusCmd := exec.Command("git", "status", "--porcelain")
	fileLines, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get files from git status: (\"%s\"):\n%s",
			cmdString, err)
	}
	files := make(map[string]struct{})
	for _, line := range bytes.Split(fileLines, []byte{'\n'}) {
		if len(line) == 0 || whitespace.Match(line) {
			continue
		}
		// Match status line against one of the regexes
		captureGroups, err := func() ([][]byte, error) {
			if c := statusPlain.FindSubmatch(line); c != nil {
				return c, nil
			}
			if c := statusArrow.FindSubmatch(line); c != nil {
				return c, nil
			}
			return nil, fmt.Errorf("No status regex matched \"%s\" line:\n%s",
				cmdString, string(line))
		}()
		if err != nil {
			return nil, err
		}

		// Skip files that are in the workding directory but haven't been added
		// to the index yet (usually logs and scripts. line starts with ??)
		if bytes.Equal(captureGroups[1], []byte{'?', '?'}) {
			continue
		}
		filename := string(captureGroups[2])
		if _, boring := alwaysModified[filename]; !boring {
			files[filename] = struct{}{}
		}
	}
	return files, nil
}

// committedFiles attempts to return the set of files (as a map from path ->
// 'true') that are different in the branches 'left' and 'right'. This is based
// on the git command 'git log --name-only' (i.e. it the list of files that are
// different is based on the list of files changed in some commit that is
// present in exactly one of the branches 'left' or 'right'
//
// All returned file paths are relative to the root of the current git repo.
// Used by modifiedFiles().
func committedFiles(left, right string) (map[string]struct{}, error) {
	// Get files only in 'left' but not 'right'
	cmd := []string{"git", "log", "--format=", "--name-only", left, "^" + right}
	cmdString := strings.Join(cmd, " ")
	logCmd := exec.Command(cmd[0], cmd[1:]...)
	leftLogLines, err := logCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get commit log (\"%s\"):\n%s", cmdString,
			err)
	}
	// Get files only in 'right' but not 'left'
	cmd = []string{"git", "log", "--format=", "--name-only", right, "^" + left}
	cmdString = strings.Join(cmd, " ")
	logCmd = exec.Command(cmd[0], cmd[1:]...)
	rightLogLines, err := logCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get commit log (\"%s\"):\n%s", cmdString,
			err)
	}

	// Dedupe files in output of command (i.e. create output set)
	files := make(map[string]struct{})
	for _, log := range [][]byte{leftLogLines, rightLogLines} {
		for _, line := range bytes.Split(log, []byte{'\n'}) {
			if len(line) > 0 {
				files[string(line)] = struct{}{}
			}
		}
	}
	return files, nil
}

// modifiedFiles returns the list of all files that are different in the
// branches 'left' and 'right'. Note that if one of the files is the current
// branch, then files that not committed will be included (i.e. files in the
// working tree)
//
// All results are file paths relative to the root of the current git repo
// (stored in GitRoot)
func modifiedFiles(left, right string) ([]string, error) {
	// Get committed files
	files, err := committedFiles(left, right)
	if err != nil {
		return nil, err
	}
	// Get uncommitted files
	if left == git.CurBranch || right == git.CurBranch {
		uncommitted, err := uncommittedFiles()
		if err != nil {
			return nil, err
		}
		for file := range uncommitted {
			files[file] = struct{}{}
		}
	}
	// Merge results
	result := make([]string, 0, len(files))
	for f := range files {
		result = append(result, f)
	}
	return result, nil
}
