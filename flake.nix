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
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

        # to work with older versions of flakes
        lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";

        # Generate a user-friendly version number.
        version = builtins.substring 0 8 lastModifiedDate;

        goTools = with pkgs; [
          go
          gopls
          delve
          golangci-lint
          mockgen
          gci
          gofumpt
        ];

      in
      rec {
        packages.default = pkgs.buildGoModule {
          pname = "bob";
          inherit version;

          src = ./.;
          vendorHash = "sha256-3coReGHbtT3snhWIh71u3T2vRPKcXIsCbUJwWRTop8M=";

          env.CGO_ENABLED = 1;

          ldflags = [
            "-X go.trai.ch/bob/internal/build.Version=${version}"
          ];

          excludePackages = [ ];
          nativeBuildInputs = [
            pkgs.mockgen
          ];

          preBuild = ''
            go generate ./...
          '';

          meta = {
            mainProgram = "bob";
          };
        };

        # So you can run `nix run .` and get bob.
        apps.default = flake-utils.lib.mkApp {
          drv = packages.default;
        };

        devShells.default = pkgs.mkShell {
          packages =
            goTools
            ++ (with pkgs; [
              nixfmt-tree
              nixfmt-rfc-style
              nixd
            ]);
        };

        # `nix fmt` will use this
        formatter = pkgs.nixfmt-rfc-style;
      }
    );
}
