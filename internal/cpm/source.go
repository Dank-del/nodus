package cpm

import (
	"fmt"
	"strings"
)

// ParseSource accepts GitHub shorthand, HTTPS URLs, and SSH URLs. CPM stores a
// canonical GitHub identity while retaining the user-provided transport URL.
func ParseSource(raw string) (Source, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return Source{}, fmt.Errorf("empty dependency source")
	}
	selector, kind := "", "default"
	if at := strings.LastIndex(input, "@"); at > strings.LastIndex(input, "/") {
		selector, kind, input = input[at+1:], "tag", input[:at]
	} else if hash := strings.LastIndex(input, "#"); hash >= 0 {
		selector, kind, input = input[hash+1:], "revision", input[:hash]
	}
	if selector == "" && kind != "default" {
		return Source{}, fmt.Errorf("source %q has an empty selector", raw)
	}

	path := input
	url := input
	switch {
	case strings.HasPrefix(input, "github.com/"):
		path = strings.TrimPrefix(input, "github.com/")
		url = "https://github.com/" + path + ".git"
	case strings.HasPrefix(input, "https://github.com/"):
		path = strings.TrimPrefix(input, "https://github.com/")
	case strings.HasPrefix(input, "http://github.com/"):
		path = strings.TrimPrefix(input, "http://github.com/")
	case strings.HasPrefix(input, "git@github.com:"):
		path = strings.TrimPrefix(input, "git@github.com:")
	case strings.HasPrefix(input, "ssh://git@github.com/"):
		path = strings.TrimPrefix(input, "ssh://git@github.com/")
	default:
		return Source{}, fmt.Errorf("unsupported source %q: MVP supports github.com repositories only", raw)
	}
	path = strings.TrimSuffix(strings.TrimSuffix(path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Source{}, fmt.Errorf("source %q must identify github.com/<owner>/<repo>", raw)
	}
	if !strings.Contains(url, "://") && !strings.HasPrefix(url, "git@") {
		url += ".git"
	}
	return Source{Host: "github.com", Owner: parts[0], Repo: parts[1], URL: url, Selector: selector, SelectorKind: kind}, nil
}
