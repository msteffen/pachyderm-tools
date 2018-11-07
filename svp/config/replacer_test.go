package config

import (
	"os"
	"testing"
)

var user string

func init() {
	user = os.Getenv("USER") // The env var I will replace (almost always set)
}

func TestBasic(t *testing.T) {
	parsed := replaceEnvVars("User is ${USER}")
	expected := "User is " + user
	if parsed != expected {
		t.Fatalf("Expected %q, but got %q", expected, parsed)
	}
}

func TestIncompleteName(t *testing.T) {
	parsed := replaceEnvVars("User is ${USER")
	expected := "User is ${USER"
	if parsed != expected {
		t.Fatalf("Expected %q, but got %q", expected, parsed)
	}
}

func TestInvalidVariable(t *testing.T) {
	parsed := replaceEnvVars("User is ${{}")
	expected := "User is ${{}"
	if parsed != expected {
		t.Fatalf("Expected %q, but got %q", expected, parsed)
	}
}

func TestNakedVariable(t *testing.T) {
	parsed := replaceEnvVars("${USER}")
	expected := user
	if parsed != expected {
		t.Fatalf("Expected %q, but got %q", expected, parsed)
	}
}
