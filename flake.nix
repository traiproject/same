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
          kessoku
        ];

        kessoku = pkgs.buildGoModule rec {
          pname = "kessoku";
          version = "1.0.0";

          src = pkgs.fetchFromGitHub {
            owner = "mazrean";
            repo = "kessoku";
            rev = "v${version}";
            sha256 = "036q7ad91qcznifs3nwgq37886db0s3b0k46kzpral2gra5bcm9a";
          };

          subPackages = [ "cmd/kessoku" ];

          ldflags = [
            "-X github.com/mazrean/kessoku/internal/config.version=v${version}"
          ];

          env.GOWORK = "off";

          vendorHash = "sha256-LC8u9tcH/Z+We698NdAtHr8ZSS0IoXrUbITPp2rr8Fk=";
        };
      in
      rec {
        packages.default = pkgs.buildGoModule {
          pname = "bob";
          inherit version;

          src = ./.;
          vendorHash = "sha256-1e8XcAAScnjcp1WLyuvg32gLxacydmEyaMhPc0NNizU=";

          env.CGO_ENABLED = 1;

          ldflags = [
            "-X go.trai.ch/bob/internal/build.Version=${version}"
          ];

          excludePackages = [ ];
          nativeBuildInputs = [
            pkgs.mockgen
            kessoku
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
