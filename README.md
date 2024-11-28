# Woosh

**Voice transcription using Whisper models.**

## Usage

Once you have the woosh server running in the background, here is what
a sample workflow would look like:

- Focus on the input field you want to type in
- Press the trigger key (currently ctrl+alt+cmd+u)
- Wait for audio cue
- Start speaking
- Release the trigger key
- Text gets typed out into the input field

## Installation

> You also could run via the nix flake using `nix run gh:meain/woosh`

- Install portaudio
- Install whisper-cpp (need to be available in path)
- Install woosh (go install github.com/meain/woosh@latest)