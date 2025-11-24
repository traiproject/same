{
  description = "Development environment with Go";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        # to work with older version of flakes
        lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";

        # Generate a user-friendly version number.
        version = builtins.substring 0 8 lastModifiedDate;

        pkgs = import nixpkgs {
          inherit system;
        };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "bob";
          version = "${version}";
          env.CGO_ENABLED = 0;
          src = ./.;
          vendorHash = "sha256-0HGkUUQuoKG8teoH5c/KC6YIx2vY0PdUBWe2duJ8lK0=";
          excludePackages = [ ];
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            nixfmt-tree
            nixfmt-rfc-style
            nixd

            go
            gopls
            delve
            golangci-lint
          ];
        };
      }
    );
}

