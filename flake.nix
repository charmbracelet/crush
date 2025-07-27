{
  description = "Crush is a tool for building software with AI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs, ... }:
    let
      allSystems = [
        "x86_64-linux" # 64-bit Intel/AMD Linux
        "aarch64-linux" # 64-bit ARM Linux
        "x86_64-darwin" # 64-bit Intel macOS
        "aarch64-darwin" # 64-bit ARM macOS
      ];
      forAllSystems =
        f:
        nixpkgs.lib.genAttrs allSystems (
          system:
          f {
            pkgs = import nixpkgs { inherit system; };
          }
        );
    in
    {
      nixosModules.default =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        let
          crushOptions = import ./nix/options.nix { inherit lib; };
        in
        {
          options = {
            programs.crush = {
              enable = lib.mkEnableOption "Enable crush";
              settings = crushOptions;
            };
          };

          config = lib.mkIf config.programs.crush.enable {
            environment.systemPackages = [ self.packages.${pkgs.system}.default ];

            environment.etc."crush/crush.json" = lib.mkIf (config.programs.crush.settings != { }) {
              text = builtins.toJSON config.programs.crush.settings;
              mode = "0644";
            };
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
          crushOptions = import ./nix/options.nix { inherit lib; };
        in
        {
          options = {
            programs.crush = {
              enable = lib.mkEnableOption "Enable crush";
              settings = crushOptions;
            };
          };

          config = lib.mkIf config.programs.crush.enable {
            home.packages = [ self.packages.${pkgs.system}.default ];

            home.file.".config/crush/crush.json" = lib.mkIf (config.programs.crush.settings != { }) {
              text = builtins.toJSON config.programs.crush.settings;
            };
          };
        };

      packages = forAllSystems (
        { pkgs }:
        let
          version = if self ? rev then self.rev else "dirty";
          crush = pkgs.buildGoModule {
            pname = "crush";
            inherit version;
            subPackages = [ "." ]; # Build from root directory
            src = self;
            vendorHash = "sha256-DoOYptWZvhAHPT9/m3jAui5KMDdZpIChJboRktM/D9U=";

            ldflags = [
              "-s"
              "-w"
              "-X github.com/charmbracelet/crush/internal/version.Version=${version}"
            ];

            nativeBuildInputs = [ pkgs.installShellFiles ];

            postInstall = ''
              installShellCompletion --cmd crush \
                --bash <($out/bin/crush completion bash) \
                --fish <($out/bin/crush completion fish) \
                --zsh <($out/bin/crush completion zsh)

              # Generate and install man page
              $out/bin/crush man > crush.1
              installManPage crush.1
            '';

            meta = with pkgs.lib; {
              description = "A tool for building software with AI";
              homepage = "https://github.com/charmbracelet/crush";
              license = licenses.mit;
              maintainers = with maintainers; [ taciturnaxolotl ];
              platforms = platforms.linux ++ platforms.darwin;
            };
          };
        in
        {
          default = crush;
        }
      );

      apps = forAllSystems (
        { pkgs }:
        {
          default = {
            type = "app";
            program = "${self.packages.${pkgs.system}.default}/bin/crush";
          };
        }
      );

      formatter = forAllSystems ({ pkgs }: pkgs.nixfmt-tree);
    };
}
