{
  description = "LeafWiki – a fast wiki for people who think in folders, not feeds";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        version = "0.0.0";

        # Build the Vite/React frontend
        ui = pkgs.buildNpmPackage {
          pname = "leafwiki-ui";
          inherit version;
          src = ./ui/leafwiki-ui;

          npmDepsHash = "sha256-gYL+VczQNEISjvNEtsfjFMFJ0tvfYgqeERlUcHdzAsY=";

          env.VITE_API_URL = "/";

          installPhase = ''
            runHook preInstall
            cp -r dist $out
            runHook postInstall
          '';
        };

        # Build the Go backend, embedding the compiled frontend
        leafwiki = pkgs.buildGoModule {
          pname = "leafwiki";
          inherit version;
          src = ./.;

          vendorHash = "sha256-/5K4BfYCFeNCJ/Sfd1eV7GO3LMx7pEe5yst7el+TfaY=";

          # Copy frontend dist into the expected location before the Go build
          preBuild = ''
            mkdir -p internal/http/dist
            cp -r ${ui}/. internal/http/dist/
          '';

          ldflags = [
            "-s"
            "-w"
            "-X github.com/perber/wiki/internal/http.EmbedFrontend=true"
            "-X github.com/perber/wiki/internal/http.Environment=production"
          ];

          # Only build the main binary; e2e-proxy is a separate Go module
          subPackages = [ "cmd/leafwiki" ];

          # modernc.org/sqlite is pure Go – no C compiler needed
          env.CGO_ENABLED = "0";
        };
      in
      {
        packages = {
          default = leafwiki;
          inherit leafwiki ui;
        };

        apps = {
          default = flake-utils.lib.mkApp { drv = leafwiki; };
          leafwiki = flake-utils.lib.mkApp { drv = leafwiki; };
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            # Go toolchain
            go
            gopls
            gotools
            golangci-lint
            # Node / frontend
            nodejs
            typescript-language-server
            # Utilities
            git
            gnumake
          ];
        };
      });
}
