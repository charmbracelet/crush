{
  description = "Crush - Interactive CLI tool for software engineering tasks";

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
            go_1_24

            # Development tools
            go-task
            golangci-lint
            gofumpt

            # Additional Go tools
            gotools
            gopls
            delve

            # Build and development utilities
            git
            gnumake

            # For profiling (used in Taskfile)
            graphviz

            # SQLite (used by the project)
            sqlite

            # ripgrep for better grep performance
            ripgrep
          ];

          # Set Go environment variables
          CGO_ENABLED = "1";
        };
      }
    );
}
