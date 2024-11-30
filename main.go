// TODO
// - Pipe output to model as we speak (whisper streaming)
// - Maybe a UI
// - Maybe a way to track all the recordings so far (not sure what the use is)

package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gordonklaus/portaudio"
	"github.com/micmonay/keybd_event"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
	"gopkg.in/yaml.v3"
)

const sampleRate = 16000     // needed for whisper
const windowSize = 2 * 16000 // 2 second window for noise floor calculation
const whisperBinary = "whisper-cpp"

type Config struct {
	// Name of the whisper model to use (TODO: optionally pass in any model)
	Model string `yaml:"model" json:"model"`
}

func readConfigFromFile(filePath string) (*Config, error) {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if stat, err := file.Stat(); err != nil || stat.IsDir() {
		return nil, nil
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func overrideConfigWithCLIArgs(config *Config) {
	flag.StringVar(
		&config.Model, "model",
		config.Model, "Name of the whisper model to use")
	flag.Parse()
}

func main() { mainthread.Init(fn) }
func fn() {
	_, err := exec.LookPath(whisperBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find binary '%s'\n", whisperBinary)
		return
	}

	// Load config from file
	configFilePath := filepath.Join(os.Getenv("HOME"), ".config", "ojut", "config.yaml")
	config, err := readConfigFromFile(configFilePath)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}

	// Override with CLI args
	if config == nil {
		config = &Config{}
	}
	overrideConfigWithCLIArgs(config)

	modelFile, err := selectModel(config.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to pick model: %s\n", err)
		return
	}

	portaudio.Initialize()
	defer portaudio.Terminate()

	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModOption, hotkey.ModCmd}, hotkey.KeyU)
	err = hk.Register()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to register hotkey: %s\n", err)
		return
	}

	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to register keyboard input: %s\n", err)
		return
	}

	defer hk.Unregister()
	fmt.Println("[Ojut is Ready]")
	fmt.Println("Model:", strings.TrimSuffix(filepath.Base(modelFile), ".bin"))

	for {
		if err := runLoop(modelFile, hk, kb); err != nil {
			log.Fatal(err)
		}
	}
}

func runLoop(modelFile string, hk *hotkey.Hotkey, kb keybd_event.KeyBonding) error {
	<-hk.Keydown()
	go playAudio()

	fmt.Fprintf(os.Stderr, "Recording...\r")
	audioBuffer := recordAudioWithDynamicNoiseFloor(hk.Keyup(), false)

	go playAudio()
	// Clear needed here as we print out noise floor data
	fmt.Fprintf(os.Stderr, "\x1b[2K\r"+"Processing...\r")

	var combinedBuffer bytes.Buffer
	header := createWAVHeader(uint32(audioBuffer.Len()))

	err := binary.Write(&combinedBuffer, binary.LittleEndian, header)
	if err != nil {
		return err
	}

	_, err = combinedBuffer.ReadFrom(audioBuffer)
	if err != nil {
		return err
	}

	cmd := exec.Command(whisperBinary, "-m", modelFile, "-f", "-", "-np", "-nt")
	cmd.Stdin = &combinedBuffer

	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start audio processing: %s\n", err)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to process audio: %s\n", err)
		fmt.Println(stderr.String())
	}

	text := strings.TrimSpace(out.String())

	// Clear line before printing
	fmt.Fprintf(os.Stderr, "\x1b[2K\r")
	fmt.Println(text)
	if cmd.Err != nil {
		fmt.Fprintf(os.Stderr, "Failed processing audio: %s\n", stderr.String())
	}

	// This is how whisper represents blank audio. Skip it, it
	// there is nothing.
	if text == "[BLANK_AUDIO]" {
		return nil
	}

	// err = typeString(text, kb)
	err = pasteString(text, kb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to type: %s\n", err)
	}

	return nil
}
