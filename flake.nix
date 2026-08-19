{
  description = "Recommend a universal offset for osu!stable from recent plays (lazer-style)";

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
            vendorHash = "sha256-kMCJDqsELXcHptr8hZ/cYgMuSwCeYi3ziIaCU9UBm0w=";
            subPackages = [ "cmd/osu-offset" ];
            env.CGO_ENABLED = "0";
            meta = {
              description = "Recommend osu!stable universal offset from recent replay hit error";
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
