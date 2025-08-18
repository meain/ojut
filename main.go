// TODO
// - Pipe output to model as we speak (whisper streaming)
// - Maybe a UI
// - Maybe a way to track all the recordings so far (not sure what the use is)

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"

	"github.com/gordonklaus/portaudio"
	"github.com/micmonay/keybd_event"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
	"gopkg.in/yaml.v3"
)

const sampleRate = 16000     // needed for whisper
const windowSize = 2 * 16000 // 2 second window for noise floor calculation
var whisperBinary = func() string {
	if binary := os.Getenv("OJUT_WHISPER_BINARY"); binary != "" {
		return binary
	}
	return "whisper-cli"
}()

type Config struct {
	// Name of the whisper model to use
	Model string `yaml:"model" json:"model"`

	// Whether to post-process text with LLM
	PostProcess bool `yaml:"post_process" json:"post_process"`

	// System prompt for LLM text processing
	LLMSystemPrompt string `yaml:"llm_system_prompt" json:"llm_system_prompt"`

	// LLM model name for post-processing
	LLMModel string `yaml:"llm_model" json:"llm_model"`

	// Base URL for LLM API
	LLMBaseURL string `yaml:"llm_base_url" json:"llm_base_url"`
}

func readDictionaryFile(filePath string) ([]string, error) {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var words []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			words = append(words, trimmed)
		}
	}
	return words, nil
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

func streamFromLLM(
	text, systemPrompt string,
	kb keybd_event.KeyBonding,
	llmConfig openai.ClientConfig,
	model string,
) error {
	client := openai.NewClientWithConfig(llmConfig)

	stream, err := client.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
			Stream: true,
		})
	if err != nil {
		return err
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if len(response.Choices) > 0 {
			content := response.Choices[0].Delta.Content
			if len(content) > 0 {
				err = pasteString(content, kb)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func overrideConfigWithCLIArgs(config *Config) *Config {
	cliConfig := &Config{}

	flag.StringVar(
		&cliConfig.Model, "model",
		"", "Name of the whisper model to use")
	flag.StringVar(
		&cliConfig.LLMModel, "llm-model",
		"", "Name of the LLM model to use for post-processing")
	flag.StringVar(
		&cliConfig.LLMBaseURL, "llm-base-url",
		"", "Base URL for LLM API")
	flag.BoolVar(
		&cliConfig.PostProcess, "post-process",
		false, "Whether to post-process text with LLM")

	flag.Parse()

	// Override config with CLI args only if they are set
	if cliConfig.Model != "" {
		config.Model = cliConfig.Model
	}

	if cliConfig.PostProcess {
		config.PostProcess = cliConfig.PostProcess
	}
	if cliConfig.LLMModel != "" {
		config.LLMModel = cliConfig.LLMModel
	}
	if cliConfig.LLMBaseURL != "" {
		config.LLMBaseURL = cliConfig.LLMBaseURL
	}

	return config
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
	config = overrideConfigWithCLIArgs(config)

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
		if err := runLoop(config, hk, kb, modelFile); err != nil {
			log.Fatal(err)
		}
	}
}

func runLoop(config *Config, hk *hotkey.Hotkey, kb keybd_event.KeyBonding, modelFile string) error {
	<-hk.Keydown()
	go playAudio()

	fmt.Fprintf(os.Stderr, "Recording...\r")
	audioBuffer := recordAudioWithDynamicNoiseFloor(hk.Keyup(), false)

	go playAudio()
	// Clear needed here as we print out noise floor data
	fmt.Fprintf(os.Stderr, "\x1b[2K\r"+"Processing...\r")

	// Read dictionary from file if it exists
	dictPath := filepath.Join(os.Getenv("HOME"), ".config", "ojut", "dictionary")
	dictionary, err := readDictionaryFile(dictPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error reading dictionary file: %w", err)
	}
	initialPrompt := strings.Join(dictionary, ", ")

	var combinedBuffer bytes.Buffer
	header := createWAVHeader(uint32(audioBuffer.Len()))

	err = binary.Write(&combinedBuffer, binary.LittleEndian, header)
	if err != nil {
		return err
	}

	_, err = combinedBuffer.ReadFrom(audioBuffer)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		whisperBinary,
		"-m",
		modelFile,
		"-f",
		"-",
		"-otxt",
		"-np",
		"-nt",
		"--prompt",
		initialPrompt)
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
	if text == "[BLANK_AUDIO]" || len(text) == 0 {
		return nil
	}

	if config.PostProcess {
		// Use default system prompt if none provided
		systemPrompt := config.LLMSystemPrompt
		if len(systemPrompt) == 0 {
			systemPrompt = "Cleanup the following transcript and add punctuation. Do not change anything else."
		}

		// Create LLM config
		apiKey := os.Getenv("OJUT_LLM_API_KEY")
		if len(apiKey) == 0 {
			apiKey = os.Getenv("OPENAI_API_KEY")
			if len(apiKey) == 0 {
				return fmt.Errorf("neither OJUT_LLM_API_KEY nor OPENAI_API_KEY environment variables are set")
			}
		}

		llmConfig := openai.DefaultConfig(apiKey)

		// Use configured base URL if available, otherwise check env var
		if config.LLMBaseURL != "" {
			llmConfig.BaseURL = config.LLMBaseURL
		} else if apiURL := os.Getenv("OJUT_LLM_ENDPOINT"); len(apiURL) > 0 {
			llmConfig.BaseURL = apiURL
		}

		// Get LLM model name
		model := config.LLMModel
		if len(model) == 0 {
			model = os.Getenv("OJUT_LLM_MODEL")
			if len(model) == 0 {
				model = "gpt-4o-mini"
			}
		}

		err = streamFromLLM(text, systemPrompt, kb, llmConfig, model)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stream from LLM: %s\n", err)
		}
	} else {
		err = pasteString(text, kb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to paste text: %s\n", err)
		}
	}

	return nil
}
