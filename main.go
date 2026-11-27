package main

import "forklift/cmd"

// Populated by goreleaser (-X main.version and friends).
var (
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func main() {
	cmd.SetVersion(version)
	_, _, _ = commit, date, builtBy
	cmd.Execute()
}
