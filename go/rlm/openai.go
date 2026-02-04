package rlm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string
	Messages    []Message
	APIBase     string
	APIKey      string
	Timeout     int
	ExtraParams map[string]interface{}
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

var (
	// defaultHTTPClient is a shared HTTP client with connection pooling
	defaultHTTPClient = &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
)

func CallChatCompletion(request ChatRequest) (string, error) {
	endpoint := buildEndpoint(request.APIBase)
	payload := map[string]interface{}{
		"model":    request.Model,
		"messages": request.Messages,
	}

	for key, value := range request.ExtraParams {
		payload[key] = value
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// Use shared client with connection pooling
	client := defaultHTTPClient
	if request.Timeout > 0 {
		// Create custom client for non-default timeout
		client = &http.Client{
			Timeout:   time.Duration(request.Timeout) * time.Second,
			Transport: defaultHTTPClient.Transport,
		}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if request.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", request.APIKey))
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", NewAPIError(resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", err
	}

	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", errors.New(parsed.Error.Message)
	}

	if len(parsed.Choices) == 0 {
		return "", errors.New("no choices returned by LLM")
	}

	return parsed.Choices[0].Message.Content, nil
}

func buildEndpoint(apiBase string) string {
	base := strings.TrimSpace(apiBase)
	if base == "" {
		base = "https://api.openai.com/v1"
	}

	if strings.Contains(base, "/chat/completions") {
		return base
	}

	return strings.TrimRight(base, "/") + "/chat/completions"
}
