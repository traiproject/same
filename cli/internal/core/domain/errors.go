package domain

import "go.trai.ch/zerr"

var (
	// ErrTaskAlreadyExists is returned when attempting to add a task with a name that already exists.
	ErrTaskAlreadyExists = zerr.New("task already exists")

	// ErrMissingDependency is returned when a task references a dependency that doesn't exist in the graph.
	ErrMissingDependency = zerr.New("missing dependency")

	// ErrMissingProjectName is returned in workspace mode when a samefile is missing a project name.
	ErrMissingProjectName = zerr.New("missing project name")

	// ErrInvalidProjectName is returned when a project name is invalid.
	ErrInvalidProjectName = zerr.New("project name can only contain alphanumeric characters, hyphens and underscores")

	// ErrDuplicateProjectName is returned when multiple projects share the same name in a workspace.
	ErrDuplicateProjectName = zerr.New("duplicate project name")

	// ErrCycleDetected is returned when a cycle is detected in the task dependency graph.
	ErrCycleDetected = zerr.New("cycle detected")

	// ErrTaskNotFound is returned when a requested task is not found in the graph.
	ErrTaskNotFound = zerr.New("task not found")

	// ErrNoTargetsSpecified is returned when no targets are specified for the run command.
	ErrNoTargetsSpecified = zerr.New("no targets specified")

	// ErrOutputPathOutsideRoot is returned when an output path is outside the project root.
	ErrOutputPathOutsideRoot = zerr.New("output path is outside project root")

	// ErrInputNotFound is returned when a declared input file or directory is not found.
	ErrInputNotFound = zerr.New("input not found")

	// ErrReservedTaskName is returned when a task uses a reserved name (e.g., "all").
	ErrReservedTaskName = zerr.New("task name 'all' is reserved")

	// ErrInvalidTaskName is returned when a task name contains invalid characters.
	ErrInvalidTaskName = zerr.New("invalid task name")

	// ErrInvalidRebuildStrategy is returned when a rebuild strategy is invalid.
	ErrInvalidRebuildStrategy = zerr.New("invalid rebuild strategy, expected 'always' or 'on-change'")

	// ErrStoreCreateFailed is returned when the build info store directory cannot be created.
	ErrStoreCreateFailed = zerr.New("failed to create build info store directory")

	// ErrStoreReadFailed is returned when the build info cannot be read.
	ErrStoreReadFailed = zerr.New("failed to read build info")

	// ErrStoreUnmarshalFailed is returned when the build info cannot be unmarshaled.
	ErrStoreUnmarshalFailed = zerr.New("failed to unmarshal build info")

	// ErrStoreMarshalFailed is returned when the build info cannot be marshaled.
	ErrStoreMarshalFailed = zerr.New("failed to marshal build info")

	// ErrStoreWriteFailed is returned when the build info cannot be written.
	ErrStoreWriteFailed = zerr.New("failed to write build info")

	// ErrConfigReadFailed is returned when the config file cannot be read.
	ErrConfigReadFailed = zerr.New("failed to read config file")

	// ErrConfigParseFailed is returned when the config file cannot be parsed.
	ErrConfigParseFailed = zerr.New("failed to parse config file")

	// ErrConfigNotFound is returned when the config file cannot be found.
	ErrConfigNotFound = zerr.New("could not find samefile or workfile")

	// ErrBuildExecutionFailed is returned when the build execution fails.
	ErrBuildExecutionFailed = zerr.New("build execution failed")

	// ErrTaskExecutionFailed is returned when a task execution fails.
	ErrTaskExecutionFailed = zerr.New("task execution failed")

	// ErrInputResolutionFailed is returned when input resolution fails.
	ErrInputResolutionFailed = zerr.New("failed to resolve inputs")

	// ErrInputHashComputationFailed is returned when input hash computation fails.
	ErrInputHashComputationFailed = zerr.New("failed to compute input hash")

	// ErrOutputHashComputationFailed is returned when output hash computation fails.
	ErrOutputHashComputationFailed = zerr.New("failed to compute output hash")

	// ErrBuildInfoUpdateFailed is returned when updating the build info store fails.
	ErrBuildInfoUpdateFailed = zerr.New("failed to update build info store")

	// ErrFailedToGetRoot is returned when the project root path cannot be determined.
	ErrFailedToGetRoot = zerr.New("failed to get absolute path of project root")

	// ErrFailedToGetOutputPath is returned when an output path cannot be determined.
	ErrFailedToGetOutputPath = zerr.New("failed to get absolute path of output")

	// ErrFailedToResolveRelativePath is returned when a relative path cannot be resolved.
	ErrFailedToResolveRelativePath = zerr.New("failed to resolve relative path")

	// ErrFailedToCleanOutput is returned when cleaning an output file fails.
	ErrFailedToCleanOutput = zerr.New("failed to clean output file")

	// ErrFileOpenFailed is returned when a file cannot be opened.
	ErrFileOpenFailed = zerr.New("failed to open file")

	// ErrFileHashFailed is returned when hashing a file fails.
	ErrFileHashFailed = zerr.New("failed to hash file content")

	// ErrPathStatFailed is returned when stating a path fails.
	ErrPathStatFailed = zerr.New("failed to stat path")

	// ErrWriteHashFailed is returned when writing the hash to the digest fails.
	ErrWriteHashFailed = zerr.New("failed to write hash to digest")

	// ErrNixCacheCreateFailed is returned when the Nix cache directory cannot be created.
	ErrNixCacheCreateFailed = zerr.New("failed to create Nix cache directory")

	// ErrNixCacheReadFailed is returned when reading from the Nix cache fails.
	ErrNixCacheReadFailed = zerr.New("failed to read from Nix cache")

	// ErrNixCacheWriteFailed is returned when writing to the Nix cache fails.
	ErrNixCacheWriteFailed = zerr.New("failed to write to Nix cache")

	// ErrNixCacheMarshalFailed is returned when marshaling Nix cache data fails.
	ErrNixCacheMarshalFailed = zerr.New("failed to marshal Nix cache data")

	// ErrNixCacheUnmarshalFailed is returned when unmarshaling Nix cache data fails.
	ErrNixCacheUnmarshalFailed = zerr.New("failed to unmarshal Nix cache data")

	// ErrNixAPIRequestFailed is returned when a NixHub API request fails.
	ErrNixAPIRequestFailed = zerr.New("failed to make NixHub API request")

	// ErrNixAPIParseFailed is returned when parsing a NixHub API response fails.
	ErrNixAPIParseFailed = zerr.New("failed to parse NixHub API response")

	// ErrNixPackageNotFound is returned when a package version is not found in NixHub.
	ErrNixPackageNotFound = zerr.New("package version not found in NixHub")

	// ErrNixInstallFailed is returned when installing a package via Nix CLI fails.
	ErrNixInstallFailed = zerr.New("failed to install package via Nix")

	// ErrMissingTool is returned when a task references a tool alias that is not defined.
	ErrMissingTool = zerr.New("tool not found")

	// ErrInvalidToolSpec is returned when a tool specification is missing the @ symbol.
	ErrInvalidToolSpec = zerr.New("invalid tool specification, expected format: package@version")

	// ErrToolResolutionFailed is returned when resolving a tool version fails.
	ErrToolResolutionFailed = zerr.New("failed to resolve tool version")

	// ErrToolInstallFailed is returned when installing a tool fails.
	ErrToolInstallFailed = zerr.New("failed to install tool")

	// ErrEnvironmentNotCached is returned when an environment should have been cached but wasn't.
	ErrEnvironmentNotCached = zerr.New("environment not found in cache")

	// ErrCacheMiss is returned when a requested item is not found in the cache.
	ErrCacheMiss = zerr.New("cache miss")
)
