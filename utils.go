package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto"
	"golang.design/x/hotkey"
)

var mu sync.Mutex

//go:embed tap.mp3
var tapAudio []byte

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
			return audioBuffer
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

			// We have to do the following computation only if we have
			// to break on silence
			if !cancelOnSilence {
				continue
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

func playAudio() error {
	d, err := mp3.NewDecoder(bytes.NewReader(tapAudio))
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
