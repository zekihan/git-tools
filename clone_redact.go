package main

import (
	"net/url"
	"regexp"
	"strings"
)

var gitURLCredentials = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s]*@`)

// safeGitURL retains only parsed connection context, never raw repository input.
func safeGitURL(value string) string {
	if matches := scpGitURLPattern.FindStringSubmatch(value); len(matches) > 0 {
		return "ssh://" + matches[1]
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.Scheme == "" || parsed.Opaque != "" {
		return "[invalid repository URL]"
	}

	return parsed.Scheme + "://" + parsed.Host
}

func redactGitURL(value string) string {
	return gitURLCredentials.ReplaceAllString(value, "${1}[redacted]@")
}

func redactCloneOutput(output, repoURL string) string {
	output = strings.ReplaceAll(output, repoURL, redactGitURL(repoURL))

	parsed, err := url.Parse(repoURL)
	if err == nil && parsed.User != nil {
		password, _ := parsed.User.Password()

		encodedUser, encodedPassword, _ := strings.Cut(parsed.User.String(), ":")
		for _, credential := range []string{
			parsed.User.String(), encodedPassword, password, encodedUser, parsed.User.Username(),
		} {
			if credential != "" {
				output = strings.ReplaceAll(output, credential, "[redacted]")
			}
		}
	}

	return redactGitURL(output)
}

func cloneDiagnostics(output *commandBuffer, repoURL string) string {
	if output.truncated {
		// The retained prefix may end inside a credential, before redaction can
		// recognize it. Omit it entirely rather than exposing partial secrets.
		return "[output truncated]\n[diagnostics omitted to protect incomplete credentials]"
	}

	return redactCloneOutput(output.String(), repoURL)
}
