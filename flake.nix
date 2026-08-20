{
  description = "Recommend a universal offset for osu!stable from live process hit error";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = lib.genAttrs systems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          osu-offset = pkgs.buildGoModule {
            pname = "osu-offset";
            version = "0.1.0";
            src = self;
            vendorHash = "sha256-wwozNBqlMnIU4EYU9SzRX41TJhwA75d0OFm2d5cACwE=";
            subPackages = [ "cmd/osu-offset" ];
            env.CGO_ENABLED = "0";
            meta = {
              description = "Watch osu!.exe and recommend Offset from live hit error";
              license = lib.licenses.gpl3Only;
              mainProgram = "osu-offset";
            };
          };
        in
        {
          inherit osu-offset;
          default = osu-offset;
        }
      );

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/osu-offset";
        };
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
            ];
          };
        }
      );
    };
}
