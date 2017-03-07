package cmds

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Op struct {
	args []string // The last command (updated after each call to Run())

	action string       // Updated as we run the command, for reporting errors
	err    error        // The error oject returned by exec.Command (or some such)
	errMsg bytes.Buffer // The text written by the last command to stderr
	output io.Writer    // The text written by the last command to stdout (if set)
}

func StartOp() *Op {
	return &Op{}
}

// LastError returns any errors produced during a Run() call.
func (o *Op) LastError() error {
	return o.err
}

// LastErrorMsg returns the stderr text from the last Run() call; this is
// mostly useful for matching against known error messages with regexes to
// handle them programmatically
func (o *Op) LastErrorMsg() []byte {
	return bytes.TrimSpace(o.errMsg.Bytes())
}

// Similar to LastError(), this produces an error with a detailed error message
func (o *Op) DetailedError() error {
	if o.errMsg.Len() > 0 {
		return fmt.Errorf("%s (command: \"%s\"):\n%s\n(%s)", o.action,
			strings.Join(o.args, " "), o.errMsg.Bytes(), o.err.Error())
	} else {
		return fmt.Errorf("%s (command: \"%s\"):\n(%s)", o.action,
			strings.Join(o.args, " "), o.err.Error())
	}
}

// If called, Op will collect the output on stdout commands it runs
func (o *Op) CollectStdOut() {
	o.output = &bytes.Buffer{}
}

func (o *Op) Output() string {
	if o.output != nil {
		if buf, ok := o.output.(*bytes.Buffer); ok {
			return buf.String()
		}
	}
	return ""
}

// If called, Op will write the output to 'w'
func (o *Op) OutputTo(w io.Writer) {
	o.output = w
}

// Run a command (assuming no previous commands have failed)
func (o *Op) Run(inputargs ...string) {
	// Only run while the whole Op is still successful
	if o.err != nil {
		return
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
	if o.output != nil {
		cmd.Stdout = o.output
	}
	if o.err = cmd.Run(); o.err != nil {
		o.action = "could not run command"
		return
	}
}
