{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils = {
      url = "github:numtide/flake-utils";
    };
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          # As of 2025-02-07, 1.23.6 is still unavailable in nixpkgs-unstable, so we need to use overlay to build 1.23.6 ourselves.
          overlays = [
            (final: prev: {
              go = (
                prev.go.overrideAttrs {
                  version = "1.23.6";
                  src = prev.fetchurl {
                    url = "https://go.dev/dl/go1.23.6.src.tar.gz";
                    hash = "sha256-A5xbBOZSedrO7opvcecL0Fz1uAF4K293xuGeLtBREiI=";
                  };
                }
              );
            })
          ];
        };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
          ];
        };
      }
    );
}
