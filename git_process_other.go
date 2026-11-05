//go:build !unix

package main

import "os/exec"

func detachGitProcess(*exec.Cmd) {}
