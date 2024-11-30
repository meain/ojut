// TODO
// - Pipe output to model as we speak (whisper streaming)
// - Maybe a UI
// - Maybe a way to track all the recordings so far (not sure what the use is)
// - Ability to set config (default model and keybinding)

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/gordonklaus/portaudio"
	"github.com/micmonay/keybd_event"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
)

const sampleRate = 16000     // needed for whisper
const windowSize = 2 * 16000 // 2 second window for noise floor calculation
const whisperBinary = "whisper-cpp"

func main() { mainthread.Init(fn) }
func fn() {
	_, err := exec.LookPath(whisperBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find binary '%s'\n", whisperBinary)
		return
	}

	modelFile, err := selectModel()
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
