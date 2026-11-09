package internal

// Version is the current application version.
// Set at build time via -ldflags from the root VERSION file.
// Falls back to "dev" for plain `go run` without ldflags.
var Version = "dev"
