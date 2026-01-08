{
  description = "The glamourous AI coding agent for your favourite terminal 💘";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forEachSystems = fn: nixpkgs.lib.genAttrs systems (system: fn nixpkgs.legacyPackages.${system});

      version = if self ? shortRev then "nightly-${self.shortRev}" else "dev";

      mkCrush =
        pkgs:
        pkgs.buildGoModule {
          pname = "crush";
          inherit version;

          src = ./.;

          proxyVendor = true;
          vendorHash = "sha256-DgAEvl0zVDEpwo6R2nHlVkC9YU2MAdQKt2MavEx1dcs=";

          env = {
            CGO_ENABLED = "0";
            GOEXPERIMENT = "greenteagc";
          };

          ldflags = [
            "-s"
            "-w"
            "-X github.com/charmbracelet/crush/internal/version.Version=${version}"
          ];

          buildFlags = [ "-trimpath" ];

          # Tests require network access to external APIs
          doCheck = false;

          nativeBuildInputs = [ pkgs.installShellFiles ];

          postInstall = ''
            # Generate shell completions
            installShellCompletion --cmd crush \
              --bash <($out/bin/crush completion bash) \
              --zsh <($out/bin/crush completion zsh) \
              --fish <($out/bin/crush completion fish)

            # Generate man pages
            mkdir -p $out/share/man/man1
            $out/bin/crush man | gzip -c > $out/share/man/man1/crush.1.gz
          '';

          meta = with pkgs.lib; {
            description = "The glamourous AI coding agent for your favourite terminal 💘";
            homepage = "https://github.com/charmbracelet/crush";
            license = licenses.fsl11Mit;
            mainProgram = "crush";
            maintainers = [ ];
            platforms = platforms.unix;
          };
        };
    in
    {
      packages = forEachSystems (pkgs: {
        default = mkCrush pkgs;
      });

      devShells = forEachSystems (pkgs: {
        default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_25
            gofumpt
            golangci-lint
            go-task
            gopls
          ];

          shellHook = ''
            export GOEXPERIMENT=greenteagc
          '';
        };
      });

      overlays.default = final: prev: {
        crush = self.packages.${prev.system}.default;
      };

      nixosModules.default =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        let
          cfg = config.programs.crush;
        in
        {
          options.programs.crush = {
            enable = lib.mkEnableOption "Crush";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.default;
              description = "The Crush package to use";
            };
          };

          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];
          };
        };

      homeManagerModules.default =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        let
          cfg = config.programs.crush;
        in
        {
          options.programs.crush = {
            enable = lib.mkEnableOption "Crush";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.default;
              description = "The Crush package to use";
            };

            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Configuration written to ~/.config/crush/config.yaml";
            };
          };

          config = lib.mkIf cfg.enable {
            home.packages = [ cfg.package ];

            xdg.configFile."crush/config.yaml" = lib.mkIf (cfg.settings != { }) {
              source = (pkgs.formats.yaml { }).generate "crush-config" cfg.settings;
            };
          };
        };
    };
}
