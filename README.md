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

## Configuration

You can specify the whisper model to use. This can be done via either the config
file or using CLI args. CLI args will override the value in the config file. We
currently only have support to specify the model, but will add more options in
the future.

Here is what the config file looks like:
```yaml
model: "medium.en-q8_0" # use "tiny.en-q8_0" if you have a slow machine
```

Here is how you would specify using the CLI:

```sh
ojut -model tiny.en-q8_0
```

_You can specify model as empty `ojut -model ""` to show a picker._

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
