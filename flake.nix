{
  description = "Crush - AI-powered coding assistant CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Go 1.25+ is required per go.mod
        go = pkgs.go_1_25;

        # Version from git
        version = if self ? shortRev then "dev-${self.shortRev}"
                  else if self ? dirtyShortRev then "dev-${self.dirtyShortRev} (dirty)"
                  else if self ? lastModifiedDate then "dev-${self.lastModifiedDate}"
                  else "dev";

        # Build the crush binary
        crush = pkgs.buildGoModule.override { inherit go; } {
          pname = "crush";
          inherit version;
          src = ./.;

          # Vendor hash for reproducible builds. To update:
          # 1. Set to empty string: vendorHash = "";
          # 2. Run: nix build 2>&1 | grep 'got:'
          # 3. Update with the hash from the error message
          vendorHash = "sha256-8d7NyNla5fSRkF5bntDBC7upq67sHVIlONmSREfUWNs=";

          env = {
            CGO_ENABLED = "0";
            GOEXPERIMENT = "greenteagc";
          };

          ldflags = [
            "-s"
            "-w"
            "-X github.com/charmbracelet/crush/internal/version.Version=${version}"
          ];

          # Skip tests during build (run separately with `task test`)
          doCheck = false;

          meta = with pkgs.lib; {
            description = "AI-powered coding assistant CLI";
            homepage = "https://github.com/charmbracelet/crush";
            license = licenses.mit;
            mainProgram = "crush";
          };
        };
      in
      {
        # `nix build`
        packages = {
          default = crush;
          crush = crush;
        };

        # `nix run`
        apps.default = flake-utils.lib.mkApp {
          drv = crush;
        };

        # `nix develop`
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            gofumpt
            go-task
            delve
            git
          ];

          env = {
            CGO_ENABLED = "0";
            GOEXPERIMENT = "greenteagc";
          };

          shellHook = ''
            echo "Crush development shell"
            echo "  go version: $(go version)"
            echo ""
            echo "Commands:"
            echo "  task build    - Build crush binary"
            echo "  task test     - Run tests"
            echo "  task lint     - Run linters"
            echo "  task lint:fix - Run linters with auto-fix"
            echo "  task fmt      - Format code with gofumpt"
            echo "  task dev      - Run with profiling enabled"
            echo "  task install  - Install to GOPATH"
          '';
        };

        # `nix flake check` - run tests (excluding agent tests that require VCR cassettes)
        checks.default = pkgs.stdenv.mkDerivation {
          name = "crush-tests";
          src = ./.;

          nativeBuildInputs = [ go ];

          env = {
            CGO_ENABLED = "0";
            GOEXPERIMENT = "greenteagc";
            GOCACHE = "/tmp/go-cache";
            GOMODCACHE = "/tmp/go-mod-cache";
          };

          buildPhase = ''
            export HOME=$TMPDIR
            # Skip agent tests - they use VCR cassettes with hardcoded paths
            # that don't work in the Nix sandbox. Run with `task test` outside Nix.
            go test $(go list ./... | grep -v '/agent$')
          '';

          installPhase = ''
            touch $out
          '';
        };
      });
}
