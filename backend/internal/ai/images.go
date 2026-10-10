// Package ai generates images from text prompts.
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ImageGenerator turns a prompt into PNG bytes.
type ImageGenerator interface {
	Generate(ctx context.Context, prompt string) ([]byte, error)
}

// OpenAIImages calls the OpenAI Images API from the server.
//
// The course version called OpenAI from the browser with the API key built
// into the JavaScript bundle (dangerouslyAllowBrowser), so anyone who opened
// the site could copy the key. Here the key never leaves the server.
type OpenAIImages struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewOpenAIImages(apiKey, model string) *OpenAIImages {
	return &OpenAIImages{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.openai.com/v1",
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type imagesRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

type imagesResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAIImages) Generate(ctx context.Context, prompt string) ([]byte, error) {
	body, _ := json.Marshal(imagesRequest{Model: o.Model, Prompt: prompt, N: 1, Size: "1024x1024"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image generation request: %w", err)
	}
	defer resp.Body.Close()

	var parsed imagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode image response (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := http.StatusText(resp.StatusCode)
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return nil, fmt.Errorf("image generation failed: %s", msg)
	}
	if len(parsed.Data) == 0 || parsed.Data[0].B64JSON == "" {
		return nil, errors.New("image generation returned no image")
	}
	return base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
}
