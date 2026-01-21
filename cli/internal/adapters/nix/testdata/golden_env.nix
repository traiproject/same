let
system = "x86_64-darwin";
flake_0 = builtins.getFlake "github:NixOS/nixpkgs/1234567890abcdef";
pkgs_0 = flake_0.legacyPackages.${system};
flake_1 = builtins.getFlake "github:NixOS/nixpkgs/fedcba0987654321";
pkgs_1 = flake_1.legacyPackages.${system};
in
pkgs_0.mkShell {
buildInputs = [
pkgs_0.go_1_25
pkgs_0.golangci-lint
pkgs_1.nodejs-18_x
];
}
