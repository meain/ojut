{
  description = "A basic go flake with devShell";

  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in
      {
        packages = rec {
          default = pkgs.buildGoModule {
            pname = "woosh";
            version = "dev";
            src = ./.;
            vendorHash = "sha256-TFI8h8wAA4OcchOpLICFbpsZ5SVhrQJ2FAzbj1Q3iyM=";
            doCheck = false;

            nativeBuildInputs = [
              pkgs.pkg-config
            ];

            buildInputs = [
              pkgs.portaudio
              pkgs.openai-whisper-cpp
            ] ++ pkgs.lib.optionals pkgs.stdenv.hostPlatform.isDarwin [
              pkgs.darwin.apple_sdk.frameworks.Cocoa
            ];

            meta = with pkgs.lib; {
              description = "Voice transcription using Whisper models";
              homepage = "https://github.com/meain/woosh";
              license = licenses.asl20;
              maintainers = with maintainers; [ meain ];
              mainProgram = "woosh";
            };
          };
        };

        devShells.default = pkgs.mkShell {
          hardeningDisable = [ "fortify" ];
          packages = with pkgs; [
            openai-whisper-cpp
            pkg-config
            portaudio
            darwin.apple_sdk.frameworks.Cocoa

            # development
            go
            gofumpt
            delve
          ];
        };
      }
    );
}
