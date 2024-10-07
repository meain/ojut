package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
)

const sampleRate = 16000
const silenceDuration = 1      // Seconds of silence to stop recording
const noiseFloorWindow = 16000 // Samples to analyze (1 seconds)

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

	// Open WAV file for writing
	file, err := os.Create("output.wav")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// WAV encoder setup
	enc := wav.NewEncoder(file, sampleRate, 16, 1, 1)
	buffer := &audio.IntBuffer{
		Format: &audio.Format{SampleRate: sampleRate, NumChannels: 1},
		Data:   []int{},
	}

	fmt.Println("Recording...")

	err = stream.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Calculate noise floor at the beginning
	fmt.Println("Calculating noise floor...")
	noiseFloor := calculateNoiseFloor(stream, in)
	fmt.Printf("Calculated noise floor: %.4f\n", noiseFloor)

	// Variables to track silence
	silenceFrames := 0

	// Recording loop
	for {
		err = stream.Read()
		if err != nil {
			log.Fatal(err)
		}

		buffer.Data = append(buffer.Data, convertInt16SliceToInt(in)...)

		// Check for silence based on noise floor
		isSilent := true
		for _, sample := range in {
			fmt.Printf("%d %f\r", silenceFrames, math.Abs(float64(sample))/math.MaxInt16)
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
		// fmt.Printf("Silence frames: %d\r", silenceFrames)

		if silenceFrames >= int(silenceDuration*float64(sampleRate)/float64(len(in))) {
			fmt.Println("Silence detected, stopping...")
			break
		}
	}

	err = stream.Stop()
	if err != nil {
		log.Fatal(err)
	}

	// Write to the WAV file
	if err := enc.Write(buffer); err != nil {
		log.Fatal(err)
	}

	// Close WAV encoder
	if err := enc.Close(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Recording saved to output.wav")
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

func convertInt16SliceToInt(s []int16) []int {
	result := make([]int, len(s))
	for i, v := range s {
		result[i] = int(v)
	}
	return result
}
