package domain

import (
	"go.trai.ch/zerr"
)

// NixPackageInfo represents the Nix-specific metadata for a resolved package
// on a particular system architecture.
type NixPackageInfo struct {
	// Owner is the GitHub repository owner (e.g., "NixOS").
	Owner InternedString

	// Repo is the GitHub repository name (e.g., "nixpkgs").
	Repo InternedString

	// Rev is the Git revision (commit SHA) pinning the exact version.
	Rev InternedString

	// Hash is the Nix hash (e.g., NAR hash) for content verification.
	Hash InternedString

	// AttrPath is the Nix attribute path to the package (e.g., "go_1_24").
	AttrPath InternedString
}

// ResolvedPackage represents a fully resolved Nix package with multi-architecture support.
// It maps system architectures to their specific Nix package information.
type ResolvedPackage struct {
	// Name is the canonical package name (e.g., "go").
	Name InternedString

	// Version is the resolved version string (e.g., "1.24.0").
	Version InternedString

	// Systems maps system architecture strings (e.g., "aarch64-darwin", "x86_64-linux")
	// to their specific Nix package metadata.
	Systems map[string]NixPackageInfo
}

// GetInfoForSystem retrieves the Nix package information for the specified system architecture.
// Returns ErrUnsupportedArchitecture if the architecture is not present in the resolved package.
func (p *ResolvedPackage) GetInfoForSystem(systemArch string) (NixPackageInfo, error) {
	info, exists := p.Systems[systemArch]
	if !exists {
		err := zerr.With(ErrUnsupportedArchitecture, "package", p.Name.String())
		err = zerr.With(err, "version", p.Version.String())
		err = zerr.With(err, "requested_arch", systemArch)
		return NixPackageInfo{}, err
	}
	return info, nil
}
