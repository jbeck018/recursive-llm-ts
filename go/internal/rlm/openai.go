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

	client := &http.Client{}
	if request.Timeout > 0 {
		client.Timeout = time.Duration(request.Timeout) * time.Second
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
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("LLM request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
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
