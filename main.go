package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"time"

	"github.com/gordonklaus/portaudio"
)

const sampleRate = 16000
const silenceDuration = 1      // Seconds of silence to stop recording
const noiseFloorWindow = 16000 // Samples to analyze (1 seconds)

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

	// Open stream for audio input
	in := make([]int16, 512)
	stream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, len(in), in)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	fmt.Fprintf(os.Stderr, "Recording...\n")

	err = stream.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Calculate noise floor at the beginning
	fmt.Fprintf(os.Stderr, "Calculating noise floor...\n")
	noiseFloor := calculateNoiseFloor(stream, in)
	fmt.Fprintf(os.Stderr, "Calculated noise floor: %.4f\n", noiseFloor)

	// Variables to track silence
	silenceFrames := 0

	// Buffer to store audio data
	var audioBuffer bytes.Buffer

	// Recording loop
	for {
		err = stream.Read()
		if err != nil {
			log.Fatal(err)
		}

		// Write audio data to buffer
		err = binary.Write(&audioBuffer, binary.LittleEndian, in)
		if err != nil {
			log.Fatal(err)
		}

		// Check for silence based on noise floor
		isSilent := true
		for _, sample := range in {
			fmt.Fprintf(os.Stderr, "%d %f\r", silenceFrames, math.Abs(float64(sample))/math.MaxInt16)
			if math.Abs(float64(sample))/math.MaxInt16 > noiseFloor*5 { // Adjust threshold multiplier
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

	// Create WAV header
	header := createWAVHeader(uint32(audioBuffer.Len()))

	// Write WAV header to stdout
	err = binary.Write(os.Stdout, binary.LittleEndian, header)
	if err != nil {
		log.Fatal(err)
	}

	// Write audio data to stdout
	_, err = io.Copy(os.Stdout, &audioBuffer)
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

// calculateNoiseFloor records and analyzes initial noise to calculate the average noise floor
func calculateNoiseFloor(stream *portaudio.Stream, in []int16) float64 {
	noiseSamples := []int16{}
	sampleCount := 0

	start := time.Now()
	for {
		err := stream.Read()
		if err != nil {
			log.Fatal(err)
		}

		noiseSamples = append(noiseSamples, in...)
		sampleCount += len(in)

		if sampleCount >= noiseFloorWindow || time.Since(start).Seconds() > 2 {
			break
		}
	}

	// Calculate the average amplitude of the noise window
	var sum float64
	for _, sample := range noiseSamples {
		sum += math.Abs(float64(sample)) / math.MaxInt16
	}

	return sum / float64(len(noiseSamples))
}
