package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/exp/maps"
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

var cacheFolder = filepath.Join(os.Getenv("HOME"), ".cache", "ojut", "models")

func selectModel(modelName string) (string, error) {
	pKeys := []string{}
	cachedModels := make(map[string]struct{})

	var models map[string]modelInfo
	err := json.Unmarshal(modelsJSON, &models)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(cacheFolder); err == nil {
		files, err := os.ReadDir(cacheFolder)
		if err != nil {
			return "", err
		}

		for _, file := range files {
			cachedModels[strings.TrimSuffix(file.Name(), ".bin")] = struct{}{}
		}
	}

	for key := range models {
		marker := ""
		if _, found := cachedModels[key]; found {
			marker = " [cached]"
		}
		pKeys = append(pKeys, fmt.Sprintf("%s [%s]%s", key, models[key].Size, marker))
	}

	slices.Sort(pKeys)
	sort.Slice(pKeys, func(i, j int) bool {
		return strings.Contains(pKeys[i], "[cached]") && !strings.Contains(pKeys[j], "[cached]")
	})

	if len(modelName) != 0 {
		if slices.Contains(maps.Keys(models), modelName) {
			return downloadModel(modelName)
		}

		// If the model is not found, assume this is a model path and
		// look up the path
		if _, err := os.Stat(modelName); err == nil {
			return modelName, nil
		}

		return "", fmt.Errorf("no model with name %s", modelName)
	} else {
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
}

func listModels() error {
	var models map[string]modelInfo
	err := json.Unmarshal(modelsJSON, &models)
	if err != nil {
		return err
	}

	cachedModels := make(map[string]struct{})
	if _, err := os.Stat(cacheFolder); err == nil {
		files, err := os.ReadDir(cacheFolder)
		if err != nil {
			return err
		}

		for _, file := range files {
			cachedModels[strings.TrimSuffix(file.Name(), ".bin")] = struct{}{}
		}
	}

	modelList := make([]string, 0, len(models))
	for key := range models {
		modelList = append(modelList, key)
	}
	slices.Sort(modelList)

	for _, key := range modelList {
		marker := ""
		if _, found := cachedModels[key]; found {
			marker = " [cached]"
		}
		fmt.Printf("%s [%s]%s\n", key, models[key].Size, marker)
	}

	return nil
}

func downloadModel(model string) (string, error) {
	url := fmt.Sprintf(downloadURLFormat, model)
	modelFilePath := filepath.Join(cacheFolder, fmt.Sprintf("%s.bin", model))
	tempFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.tmp", model))

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

	outFile, err := os.Create(tempFilePath)
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

	// Move the temp file to the final destination
	err = os.Rename(tempFilePath, modelFilePath)
	if err != nil {
		return "", err
	}

	// TODO: verify sha
	return modelFilePath, nil
}
