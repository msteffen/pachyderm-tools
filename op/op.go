package op

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Op tracks internal state of a sequence of bash commands that is intended to
// be run together.
type Op struct {
	args []string // The last command (updated after each call to Run())

	action string       // Updated as we run the command, for reporting errors
	err    error        // The error oject returned by exec.Command (or some such)
	errMsg bytes.Buffer // The text written by the last command to stderr
	output io.Writer    // The text written by the last command to stdout (if set)
	input  io.Reader    // The text read as input
}

// StartOp creates and initializes a new Op
func StartOp() *Op {
	return &Op{}
}

// LastError returns any errors produced during a Run() call (i.e. the error
// returned from the most recent call to 'Run()'. This is useful if you have a
// long chain of Run() commands and only want to check for an error at the end
// of them all.
func (o *Op) LastError() error {
	return o.err
}

// LastErrorMsg returns the stderr text from the last Run() call; this is
// mostly useful for matching against known error messages with regexes to
// handle them programmatically
func (o *Op) LastErrorMsg() []byte {
	return bytes.TrimSpace(o.errMsg.Bytes())
}

// DetailedError is similar to LastError(), but it produces a more detailed
// error message
func (o *Op) DetailedError() error {
	if o.err == nil {
		return nil
	}
	if o.errMsg.Len() > 0 {
		return fmt.Errorf("%s (command: \"%s\"):\n%s\n(%s)", o.action,
			strings.Join(o.args, " "), o.errMsg.Bytes(), o.err.Error())
	}
	return fmt.Errorf("%s (command: \"%s\"):\n(%s)", o.action,
		strings.Join(o.args, " "), o.err.Error())
}

// CollectStdOut directs 'o' to collect the output (from stdout) of commands it
// runs
func (o *Op) CollectStdOut() {
	o.output = &bytes.Buffer{}
}

// Output returns text emitted by the last command on stdout if CollectStdOut()
// was called before it ran (otherwise it returns the empty string).
func (o *Op) Output() string {
	if o.output != nil {
		if buf, ok := o.output.(*bytes.Buffer); ok {
			return buf.String()
		}
	}
	return ""
}

// OutputTo directs 'o' to pipe the output of subsequent commands to 'w'.
func (o *Op) OutputTo(w io.Writer) *Op {
	o.output = w
	return o
}

// InputFrom directs 'o' to pipe input from 'r' to subsequent cmds stdin
func (o *Op) InputFrom(r io.Reader) *Op {
	o.input = r
	return o
}

func (o *Op) Chdir(dest string) error {
	// Only run while the whole Op is still successful
	if o.err != nil {
		return o.err
	}
	o.args = []string{"cd", dest}
	if o.err = os.Chdir(dest); o.err != nil {
		o.action = "could not change directory"
	}
	return o.err
}

// Run runs a command (assuming no previous commands have failed). It returns
// whatever error was yielded by the first command to fail, or nil if all
// commands up to this point have succeeded. Note that LastError() returns the
// most recently returned value from 'Run()'
func (o *Op) Run(inputargs ...string) error {
	// Only run while the whole Op is still successful
	if o.err != nil {
		return o.err
	}
	// Prepare buffers for next command
	o.errMsg.Reset()
	if o.output != nil {
		if buf, ok := o.output.(*bytes.Buffer); ok {
			buf.Reset()
		} else {
			buf = nil
		}
	}

	// Create new exec.Command
	o.args = inputargs
	cmd := exec.Command(o.args[0], o.args[1:]...)
	cmd.Stderr = &o.errMsg
	if o.input != nil {
		cmd.Stdin = o.input
	}
	if o.output != nil {
		cmd.Stdout = o.output
	}
	if o.err = cmd.Run(); o.err != nil {
		o.action = "could not run command"
	}
	return o.err
}
