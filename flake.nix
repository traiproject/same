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
          vendorHash = "sha256-bnHZtHmdMBXqrs9Bb+x+OamXmubXcqMmPdi4atvVx8Q=";
          excludePackages = [ ];
          nativeBuildInputs = [ pkgs.mockgen ];
          preBuild = ''
            # Generate mocks for tests
            cd internal/core/ports
            mkdir -p mocks
            mockgen -source=logger.go -destination=mocks/mock_logger.go -package=mocks
            mockgen -source=executor.go -destination=mocks/mock_executor.go -package=mocks
            cd ../../..
          '';
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
            mockgen
          ];
        };
      }
    );
}
