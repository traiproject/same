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
          buf
        ];

      in
      rec {
        packages.default = pkgs.buildGoModule {
          pname = "same";
          inherit version;

          src = ./cli;
          vendorHash = "sha256-Kbw6mK+Ja47yD0fcdZBc34RQHpRmKTWY/SryTA/1iNU=";

          env.CGO_ENABLED = 0;

          ldflags = [
            "-X go.trai.ch/same/internal/build.Version=${version}"
          ];

          excludePackages = [ ];
          nativeBuildInputs = [
            pkgs.mockgen
          ];

          preBuild = ''
            go generate ./...
          '';

          meta = {
            mainProgram = "same";
          };
        };

        # So you can run `nix run .` and get same.
        apps.default = flake-utils.lib.mkApp {
          drv = packages.default;
        };

        devShells.default = pkgs.mkShell {
          packages =
            goTools
            ++ (with pkgs; [
              nodejs
              pnpm

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
