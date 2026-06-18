# Home Manager module – runs leafwiki as a systemd --user service.
# Secrets are read from plain files at service start time.
self:
{ config, lib, pkgs, ... }:
let
  cfg = config.services.leafwiki;
  # Fall back gracefully when the flake's system package is unavailable
  # (e.g. when home-manager is used standalone with a different nixpkgs).
  leafwikiPkg =
    self.packages.${pkgs.stdenv.hostPlatform.system}.leafwiki or pkgs.leafwiki;
  common = import ./common.nix {
    inherit lib;
    defaultPkg = leafwikiPkg;
    # str instead of path: supports systemd specifiers like %h
    dataDirType = lib.types.str;
    dataDirDefault = "%h/.local/share/leafwiki";
  };
in
{
  options.services.leafwiki = {
    enable = lib.mkEnableOption "LeafWiki (user service)";
  } // common.options;

  config = lib.mkIf cfg.enable {
    systemd.user.services.leafwiki = {
      Unit = {
        Description = "LeafWiki";
        After = [ "network.target" ];
      };

      Service = {
        ExecStart =
          let
            authArgs =
              if cfg.disableAuth
              then [ "--disable-auth" ]
              else [
                ''--jwt-secret=$(cat "${cfg.jwtSecretFile}")''
                ''--admin-password=$(cat "${cfg.adminPasswordFile}")''
              ];
          in
          common.mkExecStart cfg authArgs;
        Restart = "on-failure";
      };

      Install.WantedBy = [ "default.target" ];
    };
  };
}
