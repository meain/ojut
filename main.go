package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"

	"github.com/gordonklaus/portaudio"
)

const sampleRate = 16000       // Increased for better audio quality
const silenceDuration = 1      // Seconds of silence to stop recording
const noiseFloorWindow = 16000 // Samples to analyze (1 second)
const beepDuration = 0.15      // Duration of the beep sound in seconds
const beepFrequency = 980      // Frequency of the beep sound in Hz (A5 note)
const noiseFloorMultiplier = 5 // Multiplier for noise floor

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

func main() {
	// Initialize PortAudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Generate beep sound
	beep := generateBeep()

	// Calculate noise floor
	fmt.Fprintf(os.Stderr, "Calculating noise floor...\n")
	// playBeep(beep)
	noiseFloor := calculateNoiseFloor()
	playBeep(beep)
	fmt.Fprintf(os.Stderr, "Calculated noise floor: %.4f\n", noiseFloor)

	// Start recording
	fmt.Fprintf(os.Stderr, "Recording...\n")
	audioBuffer := recordAudio(noiseFloor)
	playBeep(beep)
	fmt.Fprintf(os.Stderr, "Recording completed.\n")

	// Create WAV header
	header := createWAVHeader(uint32(audioBuffer.Len()))

	// Write WAV header to stdout
	err := binary.Write(os.Stdout, binary.LittleEndian, header)
	if err != nil {
		log.Fatal(err)
	}

	// Write audio data to stdout
	_, err = io.Copy(os.Stdout, audioBuffer)
	if err != nil {
		log.Fatal(err)
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

func recordAudio(noiseFloor float64) *bytes.Buffer {
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

	// Variables to track silence
	silenceFrames := 0

	// Recording loop
	for {
		err = stream.Read()
		if err != nil {
			log.Fatal(err)
		}

		err = binary.Write(audioBuffer, binary.LittleEndian, in)
		if err != nil {
			log.Fatal(err)
		}

		// Check for silence based on noise floor
		isSilent := true
		for _, sample := range in {
			fmt.Fprintf(os.Stderr, "%d %f\r", silenceFrames, math.Abs(float64(sample))/math.MaxInt16)
			if math.Abs(float64(sample))/math.MaxInt16 > noiseFloor*noiseFloorMultiplier {
				isSilent = false
				break
			}
		}

		if isSilent {
			silenceFrames++
		} else {
			silenceFrames = 0
		}

		if silenceFrames >= int(silenceDuration*float64(sampleRate)/float64(len(in))) {
			fmt.Fprintf(os.Stderr, "Silence detected, stopping...\n")
			break
		}
	}

	err = stream.Stop()
	if err != nil {
		log.Fatal(err)
	}

	return audioBuffer
}

func calculateNoiseFloor() float64 {
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

	noiseSamples := make([]float64, noiseFloorWindow)
	sampleCount := 0

	for sampleCount < noiseFloorWindow {
		err := stream.Read()
		if err != nil {
			log.Fatal(err)
		}
		for _, sample := range in {
			if sampleCount < noiseFloorWindow {
				noiseSamples[sampleCount] = math.Abs(float64(sample)) / math.MaxInt16
				sampleCount++
			} else {
				break
			}
		}
	}

	err = stream.Stop()
	if err != nil {
		log.Fatal(err)
	}

	// Calculate the average amplitude of the noise window
	var sum float64
	for _, sample := range noiseSamples {
		sum += sample
	}

	return sum / float64(len(noiseSamples))
}

func generateBeep() []float32 {
	beepSamples := int(beepDuration * sampleRate)
	beep := make([]float32, beepSamples)

	for i := range beep {
		t := float64(i) / sampleRate
		// Apply a sine wave envelope for a smoother sound
		envelope := math.Sin(math.Pi * t / beepDuration)
		beep[i] = float32(math.Sin(2*math.Pi*beepFrequency*t) * envelope * 0.5)
	}

	return beep
}

func playBeep(beep []float32) {
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
