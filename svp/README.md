# svp

svp (short for "s'il vous plait") is a grab bag of tools for managing git,
github, source control, code reviews, etc. for my work on Pachyderm.

My main goal is to be able to work on two Pachyderm features at the same time.
Normally, the way one achieves this in git is to have two feature branches,
work in one branch until you're ready to switch, commit your code, checkout the
other branch, and then work there.  What I'd like to be able to do is have two
terminals open simultaneously, so that switching from one branch to the other
just involves switching desktops. I don't interrupt any builds or need to
remember what the relevant branches are named or which files were active. I can
just look at the other window and everything's there.

This means that I need two clones of the pachyderm repo on my computer (since
I can't otherwise have two branches checked out at the same time) which means I
need two different directories that I'm building code in, and two different
`$GOPATH`s.

One advantage of working this way is the reduction cognitive overhead. Like I
said, I don't have to remember what my two branches are called, and when
switching branches, I don't have to remember what's done and what isn't (/I
don't have to commit incomplete or incorrect code, and remember what's incomplete,
just to switch branches).

Another advantage is that it makes squashing easier. In the old workflow,
suppose you want to squash all of your commits on branch A, and then update
`master`, and then replay your commit on the updated `master` branch. Easy to
do:

```
git rebase -i master  # Squash commits to one commit
git checkout master && git pull origin master
git checkout -
git rebase master
```

When you want to do the same thing in branch B, though, this won't work. First
you need to squash all of your commits in branch B. If you don't, you end up
needing to resolve all of your merge conflicts several times: when you finish
resolving conflicts while replaying commit N, you usually have to re-resolve
the same conflicts when you replay commit (N+1). But you can't squash all of
your commits with `git rebase -i master` anymore, because master has already
advanced.  If you try, you end up engaging in this merge conflict wrestling,
which is exactly what you were trying to avoid.

With two repos, each repo can have the `master` branch at a different revision.
That means you can run the commands above in both repos to do the
squashing/replaying independently.

A final advantage is that it gives me my build time back. In the
branch-switching workflow, you can't switch branches while your code is
building (or the compiler might try to build half of your code from one branch
against half from another). So you have to wait for builds to finish before
switching branches. Admittedly, in go, this isn't too significant (our builds
usually only take a minute or so), but if build times start creeping up, I can
stay productive.

`svp` is a tool that enables this way of working. I can say `svp new-client` and
it will create a new directory in `${HOME}/clients`, set up a go workspace
there, and pull the pachyderm repo into that directory. I've already modified
`cd` on my machine to reset `$GOPATH` whenever I enter such a directory. `svp
sync` will do all of the rebasing to bring my working branch up-to-date with
`master`, and `svp save` will push ny working branch to github.

I can also run tools like `svp diff`, which lets me see all the changes I've
made in the working branch relative to `master`, whether or not those changes
have been committed. Similarly, `svp resume` opens all of the files that have
been modified in the working branch in `$EDITOR`. `svp submit` will let me
merge changes and delete the branch (and workspace) for this client. I hope to
implement `svp mail` to ask for code reviews, and potentially `svp test` to run
our integration test suite (potentially even on GCP/AWS/Azure).
