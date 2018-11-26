package cmds

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/msteffen/pachyderm-tools/svp/git"
)

// Regexes for parsing the output of 'git status' in uncommittedFiles()
var (
	fileNameWithQuotesRe    = regexp.MustCompile(`"(?:\"|[^"])+"`)
	fileNameWithoutQuotesRe = regexp.MustCompile(`[^" ]+`)
	fileNameRe              = regexp.MustCompile(fmt.Sprintf("(?:%s|%s)", fileNameWithoutQuotesRe, fileNameWithQuotesRe))
	plainStatusLineRe       = regexp.MustCompile(fmt.Sprintf("^(..) (%s)$", fileNameRe))
	arrowStatusLineRe       = regexp.MustCompile(fmt.Sprintf("^(..) %s -> (%s)$", fileNameRe, fileNameRe))
	whitespaceRe            = regexp.MustCompile("^[ \t\r\n]*$")
	slashRe                 = regexp.MustCompile(`\\`)

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
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get files from git status: (\"%s\"):\n%s",
			cmdString, err)
	}
	files := make(map[string]struct{})
	for s := bufio.NewScanner(bytes.NewReader(statusOutput)); s.Scan(); {
		if len(s.Bytes()) == 0 || whitespaceRe.Match(s.Bytes()) {
			continue
		}
		// Match status line against one of the regexes
		captureGroups, err := func() ([][]byte, error) {
			if c := plainStatusLineRe.FindSubmatch(s.Bytes()); c != nil {
				return c, nil
			}
			if c := arrowStatusLineRe.FindSubmatch(s.Bytes()); c != nil {
				return c, nil
			}
			return nil, fmt.Errorf("No status regex matched \"%s\" line:\n%s",
				cmdString, string(s.Bytes()))
		}()
		if err != nil {
			return nil, err
		}

		// Skip files that are in the workding directory but haven't been added
		// to the index yet (usually logs and scripts. line starts with ??)
		if bytes.Equal(captureGroups[1], []byte{'?', '?'}) {
			continue
		}
		var filename string
		if fileNameWithQuotesRe.Match(captureGroups[2]) {
			withoutQuotes := captureGroups[2]
			withoutQuotes = withoutQuotes[1 : len(withoutQuotes)-1] // strip quotes
			filename = string(slashRe.ReplaceAllLiteral(withoutQuotes, nil))
		} else {
			filename = string(captureGroups[2])
		}
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
//
// TODO There are two diff (or log) commands that I could use, and I'm not 100%
// committed to the one I have:
//   1) git log [--options] <commit_2>..<commit_1>
//      git log [--options] <commit_1> ^<commit_2>
//      # View changes due to commits reachable from <commit_1> but not
//      # <commit_2>
//
//   Note that the leading '^' means "exclude commits reachable from 'right' and
//   is equivalent to 'left..right'. Per 'man 7 gitrevisions':
//     The .. (two-dot) Range Notation
//         The ^r1 r2 set operation appears so often that there is a shorthand
//         for it. When you have two commits r1 and r2, you can ask for commits
//         that are reachable from r2 excluding those that are reachable from
//         r1 by ^r1 r2 and it can be written as r1..r2.
//
//   2) git diff [--options] <commit> <commit> # Currently used
//      git diff [--options] <commit>..<commit>
//      # view the changes between two arbitrary <commit>.
//
//   3) git diff [--options] <commit_1>...<commit_2> [--] [<path>...]
//      # view the changes on the branch containing and up to  <commit_2>,
//      # starting at a common ancestor of both commits. Equivalent to:
//      #   $ git diff $(git-merge-base A B) B
//
//   Note that the 'A..B' syntax for 'diff' is different from the same syntax in
//   in 'log': Per 'man 1 git-diff':
//     For a more complete list of ways to spell <commit>, see "SPECIFYING
//     REVISIONS" section in gitrevisions(7). However, "diff" is about
//     comparing two endpoints, not ranges, and the range notations
//     ("<commit>..<commit>" and "<commit>...<commit>") do not mean a range as
//     defined in the "SPECIFYING RANGES" section in gitrevisions(7).

func committedFiles(left, right string) (map[string]struct{}, error) {
	// Get files changed between 'left' and 'right'
	cmd := []string{"git", "diff", "--name-only", left, right}
	cmdString := strings.Join(cmd, " ")
	diffCmd := exec.Command(cmd[0], cmd[1:]...)
	diffCmdOutput, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Could not get commit log (\"%s\"):\n%s", cmdString,
			err)
	}

	// put files into map for deduping
	files := make(map[string]struct{})
	for s := bufio.NewScanner(bytes.NewBuffer(diffCmdOutput)); s.Scan(); {
		if len(s.Bytes()) > 0 {
			files[s.Text()] = struct{}{}
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
