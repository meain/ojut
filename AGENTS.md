This file provides guidance when working with code in this repository.

## Project Overview

Ojut is a voice dictation application that uses Whisper models for voice-to-text transcription and automatic typing. It listens for a global hotkey (Ctrl+Alt+Cmd+U), records audio while the key is held, transcribes using Whisper, and types the result into any application. It optionally supports LLM post-processing for improved punctuation and formatting.

## Common Development Commands

### Building and Running
```bash
# Build the application
go build -o ojut

# Run directly with Go
go run .

# Install to GOPATH/bin
go install

# Run with Nix (alternative installation method)
nix run github:meain/ojut

# Enter development shell with dependencies
nix develop
```

### Development Dependencies
The application requires:
- `whisper-cli` binary (from openai-whisper-cpp) in PATH
- portaudio library for audio recording
- On macOS: Cocoa framework

### Testing and Code Quality
```bash
# Format code with gofumpt (preferred over gofmt)
gofumpt -w .

# No explicit test suite currently exists
# Manual testing involves running the application and testing voice transcription
```

## Architecture Overview

### Core Components

1. **main.go** - Application entry point and main event loop
   - Hotkey registration and handling
   - Configuration loading (YAML file + CLI args + env vars)
   - Main recording/transcription/typing workflow
   - LLM post-processing integration

2. **model.go** - Whisper model management
   - Model download and caching system (~/.cache/ojut/models/)
   - Model selection UI with interactive picker
   - Embedded models.json with available Whisper variants

3. **utils.go** - Audio recording and playback utilities
   - Dynamic noise floor detection for recording start/stop
   - WAV header creation for Whisper compatibility
   - Audio feedback (embedded tap.mp3 for recording cues)
   - PortAudio integration for cross-platform audio

4. **typer.go** - Text input simulation
   - Clipboard-based text pasting (primary method)
   - Direct keystroke simulation as fallback
   - Comprehensive keyboard mapping for special characters

### Key Technologies
- **PortAudio**: Cross-platform audio I/O
- **Whisper.cpp**: Local speech recognition via external binary
- **OpenAI Go SDK**: LLM post-processing (optional)
- **Hotkey library**: Global keyboard shortcuts
- **Nix**: Development environment and packaging

### Configuration System
Configuration is loaded in this priority order:
1. CLI arguments (highest priority)
2. Environment variables
3. YAML config file (~/.config/ojut/config.yaml)
4. Built-in defaults (lowest priority)

### Audio Processing Flow
1. Hotkey pressed → Audio recording starts with cue sound
2. Dynamic noise floor calculation to detect speech
3. Hotkey released → Recording stops, processing begins
4. Audio converted to WAV format for Whisper
5. Whisper transcription with personal dictionary support
6. Optional LLM post-processing for cleanup
7. Text typed via clipboard paste method

### Model Management
- Models downloaded from HuggingFace on first use
- Cached locally in ~/.cache/ojut/models/
- Interactive picker when no model specified
- Supports both quantized and full-precision variants

## Configuration

### Personal Dictionary
Place custom vocabulary in `~/.config/ojut/dictionary` (one term per line).
This helps Whisper recognize specialized terms, names, and technical vocabulary.

### LLM Post-Processing
Optional feature requiring API key (OJUT_LLM_API_KEY or OPENAI_API_KEY).
Supports any OpenAI-compatible endpoint via OJUT_LLM_ENDPOINT.
