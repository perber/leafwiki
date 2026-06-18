# Per-system package derivations, consumed by flake-utils.lib.eachDefaultSystem.
{ pkgs, version }:
rec {
  ui = pkgs.buildNpmPackage {
    pname = "leafwiki-ui";
    inherit version;
    src = ../ui/leafwiki-ui;

    npmDepsHash = "sha256-gYL+VczQNEISjvNEtsfjFMFJ0tvfYgqeERlUcHdzAsY=";

    env.VITE_API_URL = "/";

    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -r dist/. $out/
      runHook postInstall
    '';
  };

  leafwiki = pkgs.buildGoModule {
    pname = "leafwiki";
    inherit version;
    src = ../.;

    vendorHash = "sha256-/5K4BfYCFeNCJ/Sfd1eV7GO3LMx7pEe5yst7el+TfaY=";

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
}
