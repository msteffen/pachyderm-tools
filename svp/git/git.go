package git

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/msteffen/pachyderm-tools/op"
)

var (
	// gitOnce ensures that CurBranch and Root are only initialized once
	gitOnce sync.Once

	// CurBranch is the name of the git branch that is currently checked out
	CurBranch string
	// Root is the path to the root of the current git repo
	Root string

	// This error (from the 'git' CLI) means 'svp' was not run from a git repo
	/*const */
	notAGitRepo = regexp.MustCompile("^fatal: not a git repository")
)

// CurBranch returns the name of the current branch of the git repo you're in
func initCurBranch() error {
	// Get current branch
	op := op.StartOp()
	op.CollectStdOut()
	// Currently also aliased as 'git cur-branch'
	op.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if op.LastError() != nil {
		if notAGitRepo.Match(op.LastErrorMsg()) {
			Root = "" // no error, but we're not in a git repo
			return nil
		}
		return fmt.Errorf("could not get current branch of git repo:\n%s",
			op.DetailedError())
	}
	CurBranch = strings.TrimSpace(op.Output())
	return nil
}

// Root returns the absolute path to the root of the git repo you're in
func initRoot() error {
	op := op.StartOp()
	op.CollectStdOut()
	// Currently also aliased as 'git root'
	op.Run("git", "rev-parse", "--show-toplevel")
	if op.LastError() != nil {
		if notAGitRepo.Match(op.LastErrorMsg()) {
			Root = "" // no error, but we're not in a git repo
			return nil
		}
		return fmt.Errorf("could not get root of git repo:\n%s", op.DetailedError())
	}
	Root = strings.TrimSpace(op.Output())
	return nil
}

// InitGitInfo detects if svp is being run from inside a git repo, and if so,
// initialized CurBranch and Root.  It's public so that other packages'
// init() functions can call it if they need these, but this
// package's init() function calls it as well, so non-init() code should be
// able to read Config directly
func InitGitInfo() {
	gitOnce.Do(func() {
		if err := initRoot(); err != nil {
			log.Printf("error intializing git info: %v", err)
		}
		if Root == "" {
			return // not in a git repo--don't try to initialize the branch
		}
		if err := initCurBranch(); err != nil {
			log.Printf("error intializing current git branch: %v", err)
		}
	})
}

func init() {
	InitGitInfo()
}
