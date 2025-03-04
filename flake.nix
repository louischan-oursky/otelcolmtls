{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
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
        };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = [
            # Any Go version is fine.
            pkgs.go

            (pkgs.buildGoModule {
              name = "telemetrygen";
              src = pkgs.fetchFromGitHub {
                owner = "open-telemetry";
                repo = "opentelemetry-collector-contrib";
                rev = "v0.120.1";
                sha256 = "sha256-U73wpwQrlbfnhlVIN5GcoTzl1fOcBh8zkRgbPLZvHUI=";
              };
              vendorHash = "sha256-yCCfW21gaKdXMh/0Gop/v8h/p6i2JFyN305UtOJRwP8=";
              modRoot = "./cmd/telemetrygen";
              # internal/e2etest is another go module.
              # That will confuse buildGoModule as it does not know how to build nested go module.
              # We remove it.
              prePatch = ''
                rm -r ./cmd/telemetrygen/internal/e2etest
              '';
            })
          ];
        };
      }
    );
}
