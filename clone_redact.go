package main

import (
	"net/url"
	"regexp"
	"strings"
)

var gitURLCredentials = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s]*@`)

func redactGitURL(value string) string {
	return gitURLCredentials.ReplaceAllString(value, "${1}[redacted]@")
}

func redactCloneOutput(output, repoURL string) string {
	output = strings.ReplaceAll(output, repoURL, redactGitURL(repoURL))

	parsed, err := url.Parse(repoURL)
	if err == nil && parsed.User != nil {
		password, _ := parsed.User.Password()
		for _, credential := range []string{parsed.User.String(), password, parsed.User.Username()} {
			if credential != "" {
				output = strings.ReplaceAll(output, credential, "[redacted]")
			}
		}
	}

	return redactGitURL(output)
}
