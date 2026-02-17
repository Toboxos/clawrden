{
  description = "Clawrden - The Hypervisor for Autonomous Agents";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Common version information
        version = "0.1.0";

        # Common build inputs for all Go packages
        buildGoModule = pkgs.buildGoModule.override {
          go = pkgs.go;
        };
      in
      {
        packages = {
          # Statically-linked shim binary (matches: make build-shim)
          shim = buildGoModule {
            pname = "clawrden-shim";
            inherit version;
            src = ./.;

            vendorHash = "sha256-QJIBRnIhr4C5i2TwFSvXtcaihzVBvkrjhOKxzYGaC94=";

            # Static linking for universal compatibility
            env.CGO_ENABLED = "0";
            ldflags = [ "-s" "-w" ];

            subPackages = [ "cmd/shim" ];

            meta = {
              description = "Universal shim binary for command interception";
              mainProgram = "shim";
            };
          };

          # Warden server binary (matches: make build-warden)
          warden = buildGoModule {
            pname = "clawrden-warden";
            inherit version;
            src = ./.;

            vendorHash = "sha256-QJIBRnIhr4C5i2TwFSvXtcaihzVBvkrjhOKxzYGaC94=";

            subPackages = [ "cmd/warden" ];

            meta = {
              description = "Clawrden warden server with policy engine and HITL queue";
              mainProgram = "warden";
            };
          };

          # CLI tool (matches: make build-cli)
          cli = buildGoModule {
            pname = "clawrden-cli";
            inherit version;
            src = ./.;

            vendorHash = "sha256-QJIBRnIhr4C5i2TwFSvXtcaihzVBvkrjhOKxzYGaC94=";

            subPackages = [ "cmd/cli" ];

            meta = {
              description = "CLI tool for managing Clawrden warden";
              mainProgram = "cli";
            };
          };

          # Slack bridge (matches: make build-slack-bridge)
          slack-bridge = buildGoModule {
            pname = "slack-bridge";
            inherit version;
            src = ./.;

            vendorHash = "sha256-QJIBRnIhr4C5i2TwFSvXtcaihzVBvkrjhOKxzYGaC94=";

            subPackages = [ "cmd/slack-bridge" ];

            meta = {
              description = "Slack notification bridge for Clawrden HITL approvals";
              mainProgram = "slack-bridge";
            };
          };

          # Telegram bridge (matches: make build-telegram-bridge)
          telegram-bridge = buildGoModule {
            pname = "telegram-bridge";
            inherit version;
            src = ./.;

            vendorHash = "sha256-QJIBRnIhr4C5i2TwFSvXtcaihzVBvkrjhOKxzYGaC94=";

            subPackages = [ "cmd/telegram-bridge" ];

            meta = {
              description = "Telegram notification bridge for Clawrden HITL approvals";
              mainProgram = "telegram-bridge";
            };
          };

          # Docker image for warden (matches docker-compose.yml configuration)
          warden-docker = pkgs.dockerTools.buildLayeredImage {
            name = "clawrden-warden";
            tag = "latest";

            contents = with pkgs; [
              # Base Alpine-like minimal packages
              busybox
              wget  # For healthcheck
              self.packages.${system}.warden
            ];

            config = {
              Cmd = [
                "${self.packages.${system}.warden}/bin/clawrden-warden"
                "--socket" "/var/run/clawrden/warden.sock"
                "--policy" "/etc/clawrden/policy.yaml"
                "--audit" "/var/log/clawrden/audit.log"
                "--api" ":8080"
              ];

              ExposedPorts = {
                "8080/tcp" = {};
              };

              Env = [
                "PATH=/bin:/usr/bin"
              ];

              WorkingDir = "/app";

              Labels = {
                "org.opencontainers.image.title" = "Clawrden Warden";
                "org.opencontainers.image.description" = "The Hypervisor for Autonomous Agents";
                "org.opencontainers.image.version" = version;
              };
            };
          };

          # Default package (core binaries)
          default = pkgs.symlinkJoin {
            name = "clawrden-${version}";
            paths = [
              self.packages.${system}.shim
              self.packages.${system}.warden
              self.packages.${system}.cli
            ];
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go
            gopls
            golangci-lint
            delve

            # Docker
            docker
            docker-compose

            # General dev tools
            gnumake
            git
          ];

          shellHook = ''
            echo "🛡️  Clawrden dev environment loaded"
            echo "   Go: $(go version)"
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
          '';
        };
      }
    );
}
