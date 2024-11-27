package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"

	"github.com/gordonklaus/portaudio"
)

const sampleRate = 16000
const beepDuration = 0.10
const windowSize = 2 * 16000 // 2 second window for noise floor calculation

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
	portaudio.Initialize()
	defer portaudio.Terminate()

	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModOption, hotkey.ModCmd}, hotkey.KeyP)
	err := hk.Register()
	if err != nil {
		log.Fatalf("hotkey: failed to register hotkey: %v", err)
		return
	}

	defer hk.Unregister()
	fmt.Println("[RAUS is Ready]")

	for {
		<-hk.Keydown()
		fmt.Fprintf(os.Stderr, "Recording...\r")
		audioBuffer := recordAudioWithDynamicNoiseFloor(hk.Keyup(), false)
		fmt.Fprintf(os.Stderr, "Processing...\r")

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

func generateBeep(freq int) []float32 {
	beepSamples := int(beepDuration * sampleRate)
	beep := make([]float32, beepSamples)

	for i := range beep {
		t := float64(i) / sampleRate
		// Apply a sine wave envelope for a smoother sound
		envelope := math.Sin(math.Pi * t / beepDuration)
		beep[i] = float32(math.Sin(2*math.Pi*float64(freq)*t) * envelope * 0.5)
	}

	return beep
}

func playBeep(beep []float32, volume float32) {
	// Adjusting the beep slice for 50% volume
	for i := range beep {
		beep[i] *= volume / 100
	}

	stream, err := portaudio.OpenDefaultStream(0, 1, sampleRate, len(beep), &beep)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		log.Fatal(err)
	}

	err = stream.Write()
	if err != nil {
		log.Fatal(err)
	}

	err = stream.Stop()
	if err != nil {
		log.Fatal(err)
	}
}
