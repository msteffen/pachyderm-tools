package cmds

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Regexes for parsing the output of 'git status' in uncommittedFiles()
var (
	/* const */ whitespace = regexp.MustCompile("^[ \t\r\n]*$")
	/* const */ statusPlain = regexp.MustCompile("^(..) ([^ ]*)$")
	/* const */ statusArrow = regexp.MustCompile("^(..) [^ ]* -> ([^ ]*)$")
)

// uncommittedFiles returns the list of files that have changed in the working
// tree, by parsing the output of 'git status'.
//
// All files are relative to the root of the current git repo. Used by
// ModifiedFiles()
func uncommittedFiles() (map[string]bool, error) {
	cmdString := "git status --porcelain"
	statusCmd := exec.Command("git", "status", "--porcelain")
	fileLines, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get files from git status: (\"%s\"):\n%s",
			cmdString, err)
	}
	files := make(map[string]bool)
	for _, line := range strings.Split(string(fileLines), "\n") {
		if len(line) == 0 || whitespace.MatchString(line) {
			continue
		}
		// Match status line against one of the regexes
		var captureGroups []string
		captureGroups = statusPlain.FindStringSubmatch(line)
		if captureGroups == nil {
			captureGroups = statusArrow.FindStringSubmatch(line)
		}
		if captureGroups == nil {
			return nil, fmt.Errorf("No status regex matched \"%s\" line:\n%s",
				cmdString, line)
		}

		// Skip files that are in the workding directory but haven't been added
		// to the index yet (usually logs and scripts. line starts with ??)
		if captureGroups[1] == "??" {
			continue
		}
		files[captureGroups[2]] = true
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
// Used by ModifiedFiles().
func committedFiles(left, right string) (map[string]bool, error) {
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
	files := make(map[string]bool)
	for _, log := range [][]byte{leftLogLines, rightLogLines} {
		for _, line := range strings.Split(string(log), "\n") {
			if len(line) > 0 {
				files[line] = true
			}
		}
	}
	return files, nil
}

// ModifiedFiles returns the list of all files that are different in the
// branches 'left' and 'right'. Note that if one of the files is the current
// branch, then files that not committed will be included (i.e. files in the
// working tree)
//
// All results are file paths relative to the root of the current git repo
// (stored in GitRoot)
func ModifiedFiles(left, right string) ([]string, error) {
	// Get committed files
	files, err := committedFiles(left, right)
	if err != nil {
		return nil, err
	}
	// Get uncommitted files
	if left == CurBranch || right == CurBranch {
		uncommitted, err := uncommittedFiles()
		if err != nil {
			return nil, err
		}
		for file := range uncommitted {
			files[file] = true
		}
	}
	// Merge results
	result := make([]string, len(files))
	i := 0
	for f := range files {
		result[i] = f
		i++
	}
	return result, nil
}

// ChangedFilesCommand returns a Cobra command that prints the output of
// ModifiedFiles()
func ChangedFilesCommand() *cobra.Command {
	changed := &cobra.Command{
		Use:   "changed",
		Short: "List the files that have changed between this branch and master",
		Run: boundedCommand(0, 0, func(args []string) error {
			// Sanitize 'branch' and don't run diff if 'branch' doesn't make sense
			files, err := ModifiedFiles(CurBranch, branch)
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
