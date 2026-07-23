# Shared option declarations and helpers used by both the NixOS and the
# Home Manager module. Callers must pass:
#   - lib         – the nixpkgs lib
#   - defaultPkg  – the resolved leafwiki package for the current system
#   - dataDirType – the mkOption type for dataDir (path vs. str differs)
#   - dataDirDefault – the default value for dataDir
{
  lib,
  defaultPkg,
  dataDirType,
  dataDirDefault,
}:
{
  # ---- shared option declarations -----------------------------------------
  options = {
    package = lib.mkOption {
      type = lib.types.package;
      default = defaultPkg;
      description = "The leafwiki package to use.";
    };

    host = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1";
      description = "Address to bind to.";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 8080;
      description = "Port to listen on.";
    };

    dataDir = lib.mkOption {
      type = dataDirType;
      default = dataDirDefault;
      description = "Directory to store wiki data.";
    };

    jwtSecretFile = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = "Path to a file containing the JWT secret. Required unless disableAuth is true.";
    };

    adminPasswordFile = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = "Path to a file containing the admin password. Required unless disableAuth is true.";
    };

    disableAuth = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Disable authentication (everyone can read and edit).";
    };

    extraArgs = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      description = "Additional CLI arguments passed to leafwiki.";
    };
  };

  # ---- shared helpers ------------------------------------------------------

  # Build the ExecStart string. The authArgs differ slightly between the NixOS
  # module (reads from $CREDENTIALS_DIRECTORY) and Home Manager (reads the file
  # path directly), so the caller supplies them.
  mkExecStart =
    cfg: authArgs:
    lib.escapeShellArgs (
      [
        "${cfg.package}/bin/leafwiki"
        "--host=${cfg.host}"
        "--port=${toString cfg.port}"
        "--data-dir=${cfg.dataDir}"
      ]
      ++ authArgs
      ++ cfg.extraArgs
    );
}
