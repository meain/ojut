{
  description = "A basic go flake with devShell";

  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in
      {
        packages = rec {
          default = pkgs.buildGoModule {
            pname = "ojut";
            version = "dev";
            src = ./.;
            vendorHash = "sha256-xhAEBECtfjzzBddoBvlqOPr1toZMDw/tQ1TWDHfgbYM=";
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
              homepage = "https://github.com/meain/ojut";
              license = licenses.asl20;
              maintainers = with maintainers; [ meain ];
              mainProgram = "ojut";
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
