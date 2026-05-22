{
  description = "An explorer for AIRzone's API";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs = {
        nixpkgs.follows = "nixpkgs";
        flake-utils.follows = "flake-utils";
      };
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      gomod2nix,
    }:
    let
      nixosModule =
        args@{
          pkgs,
          ...
        }:
        (import ./module.nix {
          airzonePackage = self.packages.${pkgs.system}.default;
        }) args;
    in
    (flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        callPackage = pkgs.callPackage;
        package = callPackage ./. {
          inherit (gomod2nix.legacyPackages.${system}) buildGoApplication;
        };
        go-test = pkgs.stdenvNoCC.mkDerivation {
          name = "go-test";
          dontBuild = true;
          src = ./.;
          doCheck = true;
          nativeBuildInputs = with pkgs; [
            go
            gcc
            writableTmpDirAsHomeHook
          ];
          checkPhase = ''
            go test -v ./...
          '';
          installPhase = ''
            mkdir "$out"
          '';
        };
        go-lint = pkgs.stdenvNoCC.mkDerivation {
          name = "go-lint";
          dontBuild = true;
          src = ./.;
          doCheck = true;
          nativeBuildInputs = with pkgs; [
            golangci-lint
            go
            gcc
            writableTmpDirAsHomeHook
          ];
          checkPhase = ''
            golangci-lint run
          '';
          installPhase = ''
            mkdir "$out"
          '';
        };
      in
      {
        checks = {
          inherit go-test go-lint;
        };
        packages.default = package;
        devShells.default = callPackage ./shell.nix {
          inherit (gomod2nix.legacyPackages.${system}) mkGoEnv gomod2nix;
        };
      }
    ))
    // {
      nixosModules.default = nixosModule;
      nixosModules.airzone-explorer = nixosModule;
    };
}
