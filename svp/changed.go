package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Regexes for parsing the output of 'git status' in uncommittedFiles()
var (
	/* const */ whitespace *regexp.Regexp = regexp.MustCompile("^[ \t\r\n]*$")
	/* const */ statusPlain *regexp.Regexp = regexp.MustCompile("^(..) ([^ ]*)$")
	/* const */ statusArrow *regexp.Regexp = regexp.MustCompile("^(..) [^ ]* -> ([^ ]*)$")
)

/* Get the list of files that have changed in the working tree, by parsing the
 * output of 'git status'. All files are relative to the root of the current
 * git repo
 */
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

/* Get the list of files that have changed in the current branch vs. master.
 * All files are relative to the root of the current git repo.
 */
func committedFiles() (map[string]bool, error) {
	branch, err := CurBranch()
	if err != nil {
		return nil, fmt.Errorf("Could not get current branch name, to compare "+
			"with master:\n%s", err)
	}
	if branch == "master" {
		return nil, fmt.Errorf("Error: you're alread on 'master'")
	}
	cmdString := fmt.Sprintf("git log --format= --name-only %s ^master", branch)
	logCmd := exec.Command("git", "log", "--format=", "--name-only", branch, "^master")
	logLines, err := logCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get commit log (\"%s\"):\n%s", cmdString,
			err)
	}

	// Dedupe files in output of command
	files := make(map[string]bool)
	for _, line := range strings.Split(string(logLines), "\n") {
		if len(line) == 0 {
			continue
		}
		files[line] = true
	}
	return files, nil
}

/* Get the list of all files that are different in the current branch vs. master,
 * i.e. {files that are different in the working tree}
 *                          +
 *      {files that are modified in a branch commit}
 *
 * All file paths are relateive to the root of the current git repo. Use
 * GitRoot() to determine that path.
 */
func ModifiedFiles() ([]string, error) {
	files, err := committedFiles()
	if err != nil {
		return nil, err
	}
	uncommitted, err := uncommittedFiles()
	if err != nil {
		return nil, err
	}
	for file := range uncommitted {
		files[file] = true
	}
	result := make([]string, len(files))
	i := 0
	for f := range files {
		result[i] = f
		i++
	}
	return result, nil
}

/* Cobra command that prints the output of ModifiedFiles()
 */
func ChangedFilesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "changed",
		Short: "List the files that have changed between this branch and master",
		Run: boundedCommand(0, 0, func(args []string) error {
			files, err := ModifiedFiles()
			if err != nil {
				return err
			}
			fmt.Println(strings.Join(files, "\n"))
			return nil
		}),
	}
}
