package main

import "embed"

// artFS bundles the ASCII/ANSI plant art into the binary so botany runs as a
// single self-contained executable.
//
//go:embed all:art
var artFS embed.FS
