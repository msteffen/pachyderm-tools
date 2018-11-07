package config

// replacer.go implements a tiny parser that manually replaces all variable
// references (substrings of the form "\$\{[[:letter:]].*\}") in
// Config.ClientDirectory with the value stored in the corresponding
// environment variable.  We do this here rather than going through Bash to
// avoid code injection vulnerabilities in .svpconfig (this does not do any
// arbitrary code execution)

import (
	"bytes"
	"os"
	"unicode"
)

func maybeExtractVariableName(text []rune) ([]rune, string) {
	for i := range text {
		if text[i] == '}' {
			return text[i+1:], string(text[:i])
		}
	}
	return text, ""
}

func maybeReplaceVariable(text []rune) ([]rune, string) {
	switch {
	case len(text) < 2:
		return text, ""
	case text[0] == '{' && unicode.IsLetter(text[1]):
		var name string
		newtext, name := maybeExtractVariableName(text[1:])
		if name != "" {
			return newtext, os.Getenv(name)
		}
		return text, ""
	default:
		return text, ""
	}
}

func replaceEnvVars(clientDirectoryStr string) string {
	text := []rune(clientDirectoryStr) // we want to treat 'client_directory' as a UTF-8 str
	var result bytes.Buffer
	for len(text) > 0 {
		switch text[0] {
		case '$':
			var value string
			text, value = maybeReplaceVariable(text[1:])
			if value != "" {
				result.WriteString(value)
			} else {
				result.WriteRune('$')
			}
		default:
			result.WriteRune(text[0])
			text = text[1:]
		}
	}
	return result.String()
}
