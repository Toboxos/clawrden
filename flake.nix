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
      in
      {
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
