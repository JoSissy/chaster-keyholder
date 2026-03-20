package elevenlabs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.elevenlabs.io/v1/text-to-speech"

type Client struct {
	apiKey  string
	voiceID string
	http    *http.Client
}

func New(apiKey, voiceID string) *Client {
	return &Client{
		apiKey:  apiKey,
		voiceID: voiceID,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

type ttsRequest struct {
	Text          string        `json:"text"`
	ModelID       string        `json:"model_id"`
	VoiceSettings voiceSettings `json:"voice_settings"`
}

type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
}

// TextToSpeech convierte texto a audio MP3. Máximo 2500 chars (límite free tier).
func (c *Client) TextToSpeech(text string) ([]byte, error) {
	if len(text) > 2500 {
		text = text[:2500]
	}
	body, err := json.Marshal(ttsRequest{
		Text:    text,
		ModelID: "eleven_multilingual_v2",
		VoiceSettings: voiceSettings{
			Stability:       0.4, // más expresivo y variable
			SimilarityBoost: 0.8,
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", apiBase+"/"+c.voiceID, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elevenlabs: status %d: %s", resp.StatusCode, errBody)
	}
	return io.ReadAll(resp.Body)
}
