// Pending
// - Pipe output to model as we speak
// - Add some audio cue to let folks know we are recording
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto"
	"github.com/micmonay/keybd_event"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
)

const sampleRate = 16000     // needed for whisper
const windowSize = 2 * 16000 // 2 second window for noise floor calculation
const whisperBinary = "whisper-cpp"

var mu sync.Mutex

type wavHeader struct {
	ChunkID       [4]byte
	ChunkSize     uint32
	Format        [4]byte
	Subchunk1ID   [4]byte
	Subchunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Subchunk2ID   [4]byte
	Subchunk2Size uint32
}

func main() { mainthread.Init(fn) }
func fn() {
	_, err := exec.LookPath(whisperBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find binary '%s'\n", whisperBinary)
		return
	}

	portaudio.Initialize()
	defer portaudio.Terminate()

	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModOption, hotkey.ModCmd}, hotkey.KeyP)
	err = hk.Register()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to register hotkey\n")
		return
	}

	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to register keyboard input\n")
		return
	}

	defer hk.Unregister()
	fmt.Println("[RAUS is Ready]")

	for {
		<-hk.Keydown()
		go playAudio("tap.mp3")

		fmt.Fprintf(os.Stderr, "Recording...\r")
		audioBuffer := recordAudioWithDynamicNoiseFloor(hk.Keyup(), false)

		go playAudio("tap.mp3")
		// Clear needed here as we print out noise floor data
		fmt.Fprintf(os.Stderr, "\x1b[2K\r"+"Processing...\r")

		var combinedBuffer bytes.Buffer
		header := createWAVHeader(uint32(audioBuffer.Len()))

		err := binary.Write(&combinedBuffer, binary.LittleEndian, header)
		if err != nil {
			log.Fatal(err)
		}

		_, err = combinedBuffer.ReadFrom(audioBuffer)
		if err != nil {
			log.Fatal(err)
		}

		cmd := exec.Command("whisper-cpp", "-m", "ggml-medium.en.bin", "-f", "-", "-np", "-nt")
		cmd.Stdin = &combinedBuffer

		var out bytes.Buffer
		cmd.Stdout = &out
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err = cmd.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed processing audio: %s\n", err)
		}

		err = cmd.Wait()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed processing audio: %s\n", err)
		}

		// Clear line before printing
		fmt.Fprintf(os.Stderr, "\x1b[2K\r")
		fmt.Println(strings.TrimSpace(out.String()))
		if cmd.Err != nil {
			fmt.Fprintf(os.Stderr, "Failed processing audio: %s\n", stderr.String())
		}

		err = typeString(strings.TrimSpace(out.String()), kb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to type: %s\n", err)
		}
	}
}

func createWAVHeader(dataSize uint32) wavHeader {
	return wavHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		ChunkSize:     36 + dataSize,
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1,
		NumChannels:   1,
		SampleRate:    sampleRate,
		ByteRate:      sampleRate * 2,
		BlockAlign:    2,
		BitsPerSample: 16,
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: dataSize,
	}
}

func recordAudioWithDynamicNoiseFloor(cancel <-chan hotkey.Event, cancelOnSilence bool) *bytes.Buffer {
	audioBuffer := &bytes.Buffer{}
	in := make([]int16, 512)
	stream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, len(in), in)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		log.Fatal(err)
	}

	var noiseFloor float64
	var maxNoiseFloor float64
	var sampleCount int
	var recordingStarted bool
	var silenceCount int
	window := make([]float64, windowSize)

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	// Create a channel to signal when to stop recording
	stopChan := make(chan struct{})

	// Start a goroutine to handle the SIGHUP signal
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived SIGHUP, stopping recording.\n")
		close(stopChan)
	}()

	for {
		select {
		case <-stopChan:
			if cancelOnSilence {
				return audioBuffer
			}
		case <-cancel:
			return audioBuffer
		default:
			err = stream.Read()
			if err != nil {
				log.Fatal(err)
			}

			err = binary.Write(audioBuffer, binary.LittleEndian, in)
			if err != nil {
				log.Fatal(err)
			}

			for _, sample := range in {
				amplitude := math.Abs(float64(sample)) / math.MaxInt16
				window[sampleCount%windowSize] = amplitude
				sampleCount++

				if sampleCount >= windowSize {
					currentNoiseFloor := calculateAverage(window)
					fmt.Fprintf(os.Stderr, "Current noise floor: %.4f\r", currentNoiseFloor)

					if !recordingStarted {
						if currentNoiseFloor > noiseFloor*1.5 {
							recordingStarted = true
							maxNoiseFloor = currentNoiseFloor
						}
					} else {
						if currentNoiseFloor > maxNoiseFloor {
							maxNoiseFloor = currentNoiseFloor
							silenceCount = 0
						} else if currentNoiseFloor < maxNoiseFloor*0.5 {
							silenceCount++
							if silenceCount > 5 { // Stop after 5 consecutive low-noise windows
								fmt.Fprintf(os.Stderr, "\nNoise level dipped, stopping recording.\n")
								return audioBuffer
							}
						} else {
							silenceCount = 0
						}
					}

					noiseFloor = currentNoiseFloor
				}
			}
		}
	}
}

func calculateAverage(window []float64) float64 {
	sum := 0.0
	for _, v := range window {
		sum += v
	}
	return sum / float64(len(window))
}

func playAudio(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	d, err := mp3.NewDecoder(f)
	if err != nil {
		return err
	}

	// We can only have one context at any time. This is a quick hack
	// to deal with this limitation. The audio is small enough that it
	// should not matter in most cases.
	mu.Lock()
	defer mu.Unlock()

	c, err := oto.NewContext(d.SampleRate(), 2, 2, 8192)
	if err != nil {
		return err
	}
	defer c.Close()

	p := c.NewPlayer()
	defer p.Close()

	if _, err := io.Copy(p, d); err != nil {
		return err
	}
	return nil
}
