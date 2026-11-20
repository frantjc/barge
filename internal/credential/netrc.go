package credential

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Netrc finds credentials in $NETRC, defaulting to ~/.netrc, by the Request
// URL's hostname, as curl does. Unlike curl, it ignores the "default" entry so
// that a catch-all credential isn't sent to every host barge talks to, nor
// shadows the providers after it.
type Netrc struct {
	// Path is the path to the netrc file. It defaults to $NETRC or ~/.netrc.
	Path string
}

func (Netrc) String() string {
	return "netrc"
}

func (n Netrc) Credential(_ context.Context, req *Request) (*Credential, error) {
	path := n.Path
	if path == "" {
		path = os.Getenv("NETRC")
	}
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil
		}
		path = filepath.Join(home, ".netrc")
	}

	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	hostname := strings.ToLower(req.URL.Hostname())
	for _, m := range parseNetrc(string(b)) {
		if strings.ToLower(m.machine) == hostname && m.password != "" {
			return &Credential{Username: m.login, Password: m.password}, nil
		}
	}

	return nil, nil
}

type netrcMachine struct {
	machine, login, password string
}

// parseNetrc parses the "machine" entries of a netrc file, skipping "default"
// entries and macro definitions.
func parseNetrc(s string) []netrcMachine {
	var (
		machines []netrcMachine
		cur      *netrcMachine
		lines    = strings.Split(s, "\n")
	)

	for i := 0; i < len(lines); i++ {
		tokens := netrcTokens(lines[i])
		for j := 0; j < len(tokens); j++ {
			next := func() string {
				if j+1 < len(tokens) {
					j++
					return tokens[j]
				}
				return ""
			}

			switch tokens[j] {
			case "machine":
				machines = append(machines, netrcMachine{machine: next()})
				cur = &machines[len(machines)-1]
			case "default":
				cur = nil
			case "login":
				if v := next(); cur != nil {
					cur.login = v
				}
			case "password":
				if v := next(); cur != nil {
					cur.password = v
				}
			case "account":
				next()
			case "macdef":
				// A macro definition runs until the next blank line.
				cur = nil
				for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
					i++
				}
				j = len(tokens)
			}
		}
	}

	return machines
}

// netrcTokens splits a netrc line into whitespace-separated tokens, honoring
// double quotes and backslash escapes within them, as curl does.
func netrcTokens(line string) []string {
	var (
		tokens []string
		cur    strings.Builder
		inTok  bool
		quoted bool
	)

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quoted && c == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
		case quoted && c == '"':
			quoted = false
		case !quoted && c == '"' && !inTok:
			quoted, inTok = true, true
		case !quoted && (c == ' ' || c == '\t' || c == '\r'):
			if inTok {
				tokens = append(tokens, cur.String())
				cur.Reset()
				inTok = false
			}
		default:
			cur.WriteByte(c)
			inTok = true
		}
	}

	if inTok {
		tokens = append(tokens, cur.String())
	}

	return tokens
}
