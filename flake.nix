{
  description = "LeafWiki - a fast wiki for people who think in folders, not feeds";

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
    let
      version = "0.0.0";

      perSystemOutputs = flake-utils.lib.eachDefaultSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          pkgSet = import ./nix/packages.nix { inherit pkgs version; };
          packages = {
            default = pkgSet.leafwiki;
            inherit (pkgSet) leafwiki ui;
          };
          apps = rec {
            default = leafwiki-app;
            leafwiki-app = {
              type = "app";
              program = "${pkgSet.leafwiki}/bin/leafwiki";
            };
          };
          shell = import ./nix/shell.nix { inherit pkgs; };
        in
        {
          inherit packages apps;
          inherit (shell) devShells;
          formatter = pkgs.nixfmt;
        }
      );
    in
    perSystemOutputs
    // {
      nixosModules = {
        default = import ./nix/nixos-module.nix self;
        leafwiki = import ./nix/nixos-module.nix self;
      };

      homeManagerModules = {
        default = import ./nix/home-manager-module.nix self;
        leafwiki = import ./nix/home-manager-module.nix self;
      };
    };
}
