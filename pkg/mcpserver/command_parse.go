package mcpserver

import (
	"fmt"
	"strings"
	"unicode"
)

type parseState int

const (
	parseStateBare parseState = iota
	parseStateSingleQuote
	parseStateDoubleQuote
)

func parseCommand(command string) ([]string, error) {
	var (
		args         []string
		current      strings.Builder
		state        = parseStateBare
		escaped      bool
		tokenStarted bool
	)

	flush := func() {
		if !tokenStarted {
			return
		}
		args = append(args, current.String())
		current.Reset()
		tokenStarted = false
	}

	for _, r := range command {
		switch state {
		case parseStateSingleQuote:
			tokenStarted = true
			if r == '\'' {
				state = parseStateBare
				continue
			}
			current.WriteRune(r)
		case parseStateDoubleQuote:
			tokenStarted = true
			if escaped {
				current.WriteRune(r)
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '"':
				state = parseStateBare
			default:
				current.WriteRune(r)
			}
		default:
			if escaped {
				tokenStarted = true
				current.WriteRune(r)
				escaped = false
				continue
			}
			switch {
			case unicode.IsSpace(r):
				flush()
			case r == '\'':
				tokenStarted = true
				state = parseStateSingleQuote
			case r == '"':
				tokenStarted = true
				state = parseStateDoubleQuote
			case r == '\\':
				tokenStarted = true
				escaped = true
			default:
				tokenStarted = true
				current.WriteRune(r)
			}
		}
	}

	if escaped {
		return nil, fmt.Errorf("command ends with trailing escape")
	}
	if state == parseStateSingleQuote || state == parseStateDoubleQuote {
		return nil, fmt.Errorf("command has unterminated quote")
	}

	flush()
	if len(args) == 0 {
		return nil, fmt.Errorf("command is empty")
	}
	return args, nil
}

func commandFields(command string) []string {
	args, err := parseCommand(command)
	if err != nil {
		return strings.Fields(strings.TrimSpace(command))
	}
	return args
}
