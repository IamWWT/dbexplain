// Package version provides the build-time version for dbexplain.
// Set via ldflags during build: -X github.com/IamWWT/dbexplain/internal/version.Version=v0.1.2
package version

// Version is the current dbexplain version. Overridden at build time via -ldflags -X.
var Version = "dev"
