// Copyright 2025 R5 Labs
// This file is part of the R5 Core library.
//
// This software is provided "as is", without warranty of any kind,
// express or implied, including but not limited to the warranties
// of merchantability, fitness for a particular purpose and
// noninfringement. In no event shall the authors or copyright
// holders be liable for any claim, damages, or other liability,
// whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or
// other dealings in the software.

// Package version implements reading of build version information.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/r5-labs/r5-core/client/params"
)

const ourPath = "github.com/r5-labs/r5-core/client" // Path to our module

// These variables are set at build-time by the linker when the build is
// done by build/ci.go.
var gitCommit, gitDate string

// VCSInfo represents the git repository state.
type VCSInfo struct {
	Commit string // head commit hash
	Date   string // commit time in YYYYMMDD format
	Dirty  bool
}

// VCS returns version control information of the current executable.
func VCS() (VCSInfo, bool) {
	if gitCommit != "" {
		// Use information set by the build script if present.
		return VCSInfo{Commit: gitCommit, Date: gitDate}, true
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if buildInfo.Main.Path == ourPath {
			return buildInfoVCS(buildInfo)
		}
	}
	return VCSInfo{}, false
}

// ClientName creates a software name/version identifier according to common
// conventions in the Ethereum p2p network.
func ClientName(clientIdentifier string) string {
	git, _ := VCS()
	return fmt.Sprintf("%s/v%v/%v-%v/%v",
		strings.Title(clientIdentifier),
		params.VersionWithCommit(git.Commit, git.Date),
		runtime.GOOS, runtime.GOARCH,
		runtime.Version(),
	)
}

// runtimeInfo returns build and platform information about the current binary.
//
// If the package that is currently executing is a prefixed by our go-ethereum
// module path, it will print out commit and date VCS information. Otherwise,
// it will assume it's imported by a third-party and will return the imported
// version and whether it was replaced by another module.
func Info() (version, vcs string) {
	version = params.VersionWithMeta
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return version, ""
	}
	version = versionInfo(buildInfo)
	if status, ok := VCS(); ok {
		modified := ""
		if status.Dirty {
			modified = " (dirty)"
		}
		commit := status.Commit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		vcs = commit + "-" + status.Date + modified
	}
	return version, vcs
}

// versionInfo returns version information for the currently executing
// implementation.
//
// Depending on how the code is instantiated, it returns different amounts of
// information. If it is unable to determine which module is related to our
// package it falls back to the hardcoded values in the params package.
func versionInfo(info *debug.BuildInfo) string {
	// If the main package is from our repo, prefix version with "geth".
	if strings.HasPrefix(info.Path, ourPath) {
		return fmt.Sprintf("geth %s", info.Main.Version)
	}
	// Not our main package, so explicitly print out the module path and
	// version.
	var version string
	if info.Main.Path != "" && info.Main.Version != "" {
		// These can be empty when invoked with "go run".
		version = fmt.Sprintf("%s@%s ", info.Main.Path, info.Main.Version)
	}
	mod := findModule(info, ourPath)
	if mod == nil {
		// If our module path wasn't imported, it's unclear which
		// version of our code they are running. Fallback to hardcoded
		// version.
		return version + fmt.Sprintf("geth %s", params.VersionWithMeta)
	}
	// Our package is a dependency for the main module. Return path and
	// version data for both.
	version += fmt.Sprintf("%s@%s", mod.Path, mod.Version)
	if mod.Replace != nil {
		// If our package was replaced by something else, also note that.
		version += fmt.Sprintf(" (replaced by %s@%s)", mod.Replace.Path, mod.Replace.Version)
	}
	return version
}

// findModule returns the module at path.
func findModule(info *debug.BuildInfo, path string) *debug.Module {
	if info.Path == ourPath {
		return &info.Main
	}
	for _, mod := range info.Deps {
		if mod.Path == path {
			return mod
		}
	}
	return nil
}
