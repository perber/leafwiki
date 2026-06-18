# NixOS module – runs leafwiki as a hardened system-level systemd service.
# Secret files are loaded via systemd LoadCredential so they never appear
# in the Nix store or the unit file.
self:
{ config, lib, pkgs, ... }:
let
  cfg = config.services.leafwiki;
  leafwikiPkg = self.packages.${pkgs.stdenv.hostPlatform.system}.leafwiki;
  common = import ./common.nix {
    inherit lib;
    defaultPkg = leafwikiPkg;
    dataDirType = lib.types.path;
    dataDirDefault = "/var/lib/leafwiki";
  };
in
{
  options.services.leafwiki = {
    enable = lib.mkEnableOption "LeafWiki";

    user = lib.mkOption {
      type = lib.types.str;
      default = "leafwiki";
      description = "User account under which leafwiki runs.";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "leafwiki";
      description = "Group under which leafwiki runs.";
    };
  } // common.options;

  config = lib.mkIf cfg.enable {
    users.users.${cfg.user} = {
      isSystemUser = true;
      group = cfg.group;
      home = cfg.dataDir;
      createHome = true;
    };
    users.groups.${cfg.group} = {};

    systemd.services.leafwiki = {
      description = "LeafWiki";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      serviceConfig = {
        User = cfg.user;
        Group = cfg.group;
        WorkingDirectory = cfg.dataDir;
        Restart = "on-failure";

        ExecStart =
          let
            authArgs =
              if cfg.disableAuth
              then [ "--disable-auth" ]
              else [
                # LoadCredential exposes secrets under $CREDENTIALS_DIRECTORY
                ''--jwt-secret=$(cat "$CREDENTIALS_DIRECTORY/jwt-secret")''
                ''--admin-password=$(cat "$CREDENTIALS_DIRECTORY/admin-password")''
              ];
          in
          common.mkExecStart cfg authArgs;

        LoadCredential = lib.optionals (!cfg.disableAuth) [
          "jwt-secret:${cfg.jwtSecretFile}"
          "admin-password:${cfg.adminPasswordFile}"
        ];

        # Hardening
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ReadWritePaths = [ cfg.dataDir ];
      };
    };
  };
}
