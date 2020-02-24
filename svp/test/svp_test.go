package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewClient(t *testing.T) {
	dir, err := ioutil.TempDir(".", "svp-test-directory-")
	if err != nil {
		// t
	}
	err = os.Chdir(dir)

}
