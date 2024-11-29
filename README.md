# Ojut

**Voice dictation using Whisper models.**

## Usage

Once you have the ojut server running in the background, here is what
a sample workflow would look like:

- Focus on the input field you want to type in
- Press the trigger key (currently ctrl+alt+cmd+u)
- Wait for audio cue
- Start speaking
- Release the trigger key
- Text gets typed out into the input field

## Installation

> You also could run via the nix flake using `nix run github:meain/ojut`

- Install portaudio
- Install whisper-cpp (need to be available in path)
- Install ojut (go install github.com/meain/ojut@latest)

## What is with the name?

I'm glad you asked. It's very stupid but here is how I got to the
name. "Dictation" in Japanese is "口述筆記" which sounds like "Kōjutsu
hikki". Now I just took part of the first word "ojut". It was unique
enough and I was tired of looking for the name and so I decided I'm
just going to use it.
