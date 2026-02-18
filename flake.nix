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

        # Common version information - SINGLE SOURCE OF TRUTH
        version = "0.1.0";

        # Common vendorHash - SINGLE SOURCE OF TRUTH
        # Update with: nix build 2>&1 | grep "got:" | awk '{print $2}'
        vendorHash = "sha256-QJIBRnIhr4C5i2TwFSvXtcaihzVBvkrjhOKxzYGaC94=";

        # Base derivation that builds all binaries using Make
        # This delegates to Makefile for build logic while using buildGoModule for deps
        clawrden-build = pkgs.buildGoModule {
          pname = "clawrden";
          inherit version;
          src = ./.;
          inherit vendorHash;

          nativeBuildInputs = [ pkgs.gnumake ];

          # Override the default Go build to use Make instead
          buildPhase = ''
            runHook preBuild

            # Makefile is the source of truth for build logic
            make build-all

            runHook postBuild
          '';

          installPhase = ''
            runHook preInstall

            mkdir -p $out/bin
            cp bin/* $out/bin/

            runHook postInstall
          '';

          # Don't check subPackages since we're building everything
          subPackages = null;

          meta = {
            description = "Clawrden - The Hypervisor for Autonomous Agents";
            platforms = pkgs.lib.platforms.linux;
          };
        };
      in
      {
        packages = {
          # Individual binaries extracted from main build
          shim = pkgs.runCommand "clawrden-shim" {} ''
            mkdir -p $out/bin
            cp ${clawrden-build}/bin/clawrden-shim $out/bin/
          '';

          warden = pkgs.runCommand "clawrden-warden" {} ''
            mkdir -p $out/bin
            cp ${clawrden-build}/bin/clawrden-warden $out/bin/
          '';

          cli = pkgs.runCommand "clawrden-cli" {} ''
            mkdir -p $out/bin
            cp ${clawrden-build}/bin/clawrden-cli $out/bin/
          '';

          slack-bridge = pkgs.runCommand "slack-bridge" {} ''
            mkdir -p $out/bin
            cp ${clawrden-build}/bin/slack-bridge $out/bin/
          '';

          telegram-bridge = pkgs.runCommand "telegram-bridge" {} ''
            mkdir -p $out/bin
            cp ${clawrden-build}/bin/telegram-bridge $out/bin/
          '';

          # Docker image with all services (warden + bridges)
          # Can run single service or multiple services in one container
          warden-docker = pkgs.dockerTools.buildLayeredImage {
            name = "clawrden-warden";
            tag = "latest";

            contents = with pkgs; [
              # Base Alpine-like minimal packages
              busybox
              wget  # For healthcheck
              # All binaries in single image
              self.packages.${system}.warden
              self.packages.${system}.slack-bridge
              self.packages.${system}.telegram-bridge
            ];

            config = {
              Entrypoint = [ "${pkgs.writeScript "entrypoint.sh" ''
                #!/bin/sh
                set -e

                # Multi-service support: launch all requested services
                PIDS=""
                trap 'kill $PIDS 2>/dev/null; exit' TERM INT

                for service in "$@"; do
                  case "$service" in
                    warden)
                      ${self.packages.${system}.warden}/bin/clawrden-warden \
                        --socket /var/run/clawrden/warden.sock \
                        --policy /etc/clawrden/policy.yaml \
                        --audit /var/log/clawrden/audit.log \
                        --api :8080 &
                      PIDS="$PIDS $!"
                      ;;
                    slack)
                      ${self.packages.${system}.slack-bridge}/bin/slack-bridge \
                        --warden-url http://localhost:8080 &
                      PIDS="$PIDS $!"
                      ;;
                    telegram)
                      ${self.packages.${system}.telegram-bridge}/bin/telegram-bridge \
                        --warden-url http://localhost:8080 &
                      PIDS="$PIDS $!"
                      ;;
                    *)
                      echo "Unknown service: $service"
                      echo "Usage: $0 [warden] [slack] [telegram]"
                      exit 1
                      ;;
                  esac
                done

                # Wait for all background processes
                [ -z "$PIDS" ] && { echo "No services specified"; exit 1; }
                wait $PIDS
              ''}" ];

              Cmd = [ "warden" ];  # Default: warden only

              ExposedPorts = {
                "8080/tcp" = {};
              };

              Env = [
                "PATH=/bin:/usr/bin"
              ];

              WorkingDir = "/app";

              Labels = {
                "org.opencontainers.image.title" = "Clawrden Warden";
                "org.opencontainers.image.description" = "The Hypervisor for Autonomous Agents - Multi-service image";
                "org.opencontainers.image.version" = version;
              };
            };
          };

          # Default package (all binaries)
          default = clawrden-build;
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
