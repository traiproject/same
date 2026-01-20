package domain

import "path/filepath"

const (
	// SameDirName is the name of the internal workspace directory.
	SameDirName = ".same"

	// StoreDirName is the name of the content addressable store directory.
	StoreDirName = "store"

	// CacheDirName is the name of the cache directory.
	CacheDirName = "cache"

	// NixHubDirName is the name of the NixHub cache directory.
	NixHubDirName = "nixhub"

	// EnvDirName is the name of the environment cache directory.
	EnvDirName = "environments"

	// SameFileName is the name of the project configuration file.
	SameFileName = "same.yaml"

	// WorkFileName is the name of the workspace configuration file.
	WorkFileName = "same.work.yaml"

	// DebugLogFile is the name of the debug log file.
	DebugLogFile = "debug.log"

	// DirPerm is the default permission for directories (rwxr-x---).
	DirPerm = 0o750

	// FilePerm is the default permission for files (rw-r--r--).
	FilePerm = 0o644

	// PrivateFilePerm is the default permission for private files (rw-------).
	PrivateFilePerm = 0o600
)

// DefaultSamePath returns the default root directory for same metadata.
func DefaultSamePath() string {
	return SameDirName
}

// DefaultStorePath returns the default path for the content addressable store.
// It joins .same and store.
func DefaultStorePath() string {
	return filepath.Join(SameDirName, StoreDirName)
}

// DefaultNixHubCachePath returns the default path for the NixHub cache.
// It joins .same, cache, and nixhub.
func DefaultNixHubCachePath() string {
	return filepath.Join(SameDirName, CacheDirName, NixHubDirName)
}

// DefaultEnvCachePath returns the default path for the environment cache.
// It joins .same, cache, and environments.
func DefaultEnvCachePath() string {
	return filepath.Join(SameDirName, CacheDirName, EnvDirName)
}

// DefaultDebugLogPath returns the default path for the debug log.
// It joins .same and debug.log.
func DefaultDebugLogPath() string {
	return filepath.Join(SameDirName, DebugLogFile)
}
