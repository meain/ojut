package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/exp/slices"
)

type modelInfo struct {
	Size string `json:"size"`
	Sha  string `json:"sha"`
}

//go:embed models.json
var modelsJSON []byte

// This file mostly helps with downloading and caching models
// What about tdrz models?
const downloadURLFormat = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-%s.bin"

var cacheFolder = filepath.Join(os.Getenv("HOME"), ".cache", "woosh", "models")

func selectModel() (string, error) {
	var models map[string]modelInfo
	err := json.Unmarshal(modelsJSON, &models)
	if err != nil {
		return "", err
	}

	pKeys := []string{}
	for key := range models {
		pKeys = append(pKeys, fmt.Sprintf("%s [%s]", key, models[key].Size))
	}

	slices.Sort(pKeys)

	prompt := promptui.Select{
		Label:        "Select model",
		Items:        pKeys,
		HideSelected: true,
	}

	_, result, err := prompt.Run()

	if err != nil {
		return "", err
	}

	return downloadModel(strings.Split(result, " ")[0])
}

func downloadModel(model string) (string, error) {
	url := fmt.Sprintf(downloadURLFormat, model)
	modelFilePath := filepath.Join(cacheFolder, fmt.Sprintf("%s.bin", model))

	if _, err := os.Stat(modelFilePath); err == nil {
		return modelFilePath, nil
	}

	// Make an HTTP GET request
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	// Get the file size from headers
	size := response.ContentLength

	err = os.MkdirAll(cacheFolder, os.ModePerm)
	if err != nil {
		return "", err
	}

	outFile, err := os.Create(modelFilePath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	// Create a progress bar
	bar := progressbar.DefaultBytes(size, model)

	// Copy the response body to the file with a progress bar
	_, err = io.Copy(io.MultiWriter(outFile, bar), response.Body)
	if err != nil {
		return "", err
	}

	// TODO: verify sha
	return modelFilePath, nil
}
