package tts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiURL = "https://texttospeech.googleapis.com/v1/text:synthesize"

type Client struct {
	apiKey string
	voice  string
	http   *http.Client
}

// New crea un cliente de Google Cloud TTS.
// voice: nombre de voz Neural2, ej. "es-US-Neural2-B" (male) o "es-US-Neural2-A" (female).
func New(apiKey, voice string) *Client {
	if voice == "" {
		voice = "es-US-Neural2-B" // voz masculina por defecto (Papi)
	}
	return &Client{
		apiKey: apiKey,
		voice:  voice,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

type synthesizeRequest struct {
	Input       ttsInput       `json:"input"`
	Voice       ttsVoice       `json:"voice"`
	AudioConfig ttsAudioConfig `json:"audioConfig"`
}

type ttsInput struct {
	Text string `json:"text"`
}

type ttsVoice struct {
	LanguageCode string `json:"languageCode"`
	Name         string `json:"name"`
}

type ttsAudioConfig struct {
	AudioEncoding string  `json:"audioEncoding"`
	SpeakingRate  float64 `json:"speakingRate"`
	Pitch         float64 `json:"pitch"`
}

type synthesizeResponse struct {
	AudioContent string `json:"audioContent"`
}

// TextToSpeech convierte texto a audio MP3.
func (c *Client) TextToSpeech(text string) ([]byte, error) {
	if len(text) > 4500 {
		text = text[:4500]
	}

	langCode := "es-US"
	if len(c.voice) >= 5 {
		langCode = c.voice[:5] // "es-US" o "es-ES"
	}

	body, err := json.Marshal(synthesizeRequest{
		Input: ttsInput{Text: text},
		Voice: ttsVoice{
			LanguageCode: langCode,
			Name:         c.voice,
		},
		AudioConfig: ttsAudioConfig{
			AudioEncoding: "MP3",
			SpeakingRate:  1.0,
			Pitch:         0.0,
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL+"?key="+c.apiKey, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("google tts: status %d: %s", resp.StatusCode, respBody)
	}

	var result synthesizeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("google tts: parse error: %w", err)
	}
	if result.AudioContent == "" {
		return nil, fmt.Errorf("google tts: respuesta vacía")
	}

	return base64.StdEncoding.DecodeString(result.AudioContent)
}
