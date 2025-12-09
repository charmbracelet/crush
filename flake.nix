{
  description = "Crush development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
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
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go_1_25

            # Development tools
            gopls # Go language server
            golangci-lint # Linter
            gofumpt # Formatter (stricter than gofmt)
            go-task # Task runner
            delve # Go debugger

            # Additional tools
            git # Version control
            gh # GitHub CLI
          ];

          shellHook = ''
            # Set Go environment variables
            export CGO_ENABLED=0
            export GOEXPERIMENT=greenteagc
          '';
        };
      }
    );
}
