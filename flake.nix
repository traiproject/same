{
  description = "Development environment for Rust";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    fenix = {
      url = "github:nix-community/fenix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      fenix,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        toolchain = fenix.packages.${system}.fromToolchainFile {
          file = ./rust-toolchain.toml;
          sha256 = "sha256-qqF33vNuAdU5vua96VKVIwuc43j4EFeEXbjQ6+l4mO4=";
        };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            toolchain

            buf
          ];

          RUST_SRC_PATH = "${toolchain}/lib/rustlib/src/rust/library";
        };

        formatter = pkgs.nixfmt;
      }
    );
}
