{ pkgs }:
{
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
}
