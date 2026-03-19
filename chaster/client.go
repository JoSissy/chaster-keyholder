package chaster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"chaster-keyholder/models"
)

const baseURL = "https://api.chaster.app"

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("chaster API error %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

// GetActiveLock obtiene la sesión de castidad activa (locked y no lista para desbloquear)
func (c *Client) GetActiveLock() (*models.ChasterLock, error) {
	data, err := c.doRequest("GET", "/locks", nil)
	if err != nil {
		return nil, err
	}

	var locks []struct {
		models.ChasterLock
		IsReadyToUnlock bool   `json:"isReadyToUnlock"`
		CanBeUnlocked   bool   `json:"canBeUnlocked"`
		StartDateRaw    string `json:"startDate"`
	}
	if err := json.Unmarshal(data, &locks); err != nil {
		return nil, err
	}

	for i := range locks {
		if locks[i].Status == "locked" && !locks[i].IsReadyToUnlock {
			formats := []string{
				"2006-01-02T15:04:05.000Z",
				"2006-01-02T15:04:05Z",
				time.RFC3339,
			}
			for _, layout := range formats {
				if t, err := time.Parse(layout, locks[i].StartDateRaw); err == nil {
					locks[i].ChasterLock.StartDate = t
					break
				}
			}
			result := locks[i].ChasterLock
			return &result, nil
		}
	}

	return nil, fmt.Errorf("no hay sesión activa de castidad")
}

// AddTime añade tiempo a la sesión (segundos, puede ser negativo)
func (c *Client) AddTime(lockID string, seconds int) error {
	payload := map[string]int{"duration": seconds}
	_, err := c.doRequest("POST", fmt.Sprintf("/locks/%s/update-time", lockID), payload)
	return err
}

// UploadCombinationImage sube la foto del candado y devuelve el combinationId
func (c *Client) UploadCombinationImage(imageBytes []byte, mimeType string) (string, error) {
	// Construir multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Determinar extensión
	ext := "jpg"
	if mimeType == "image/png" {
		ext = "png"
	} else if mimeType == "image/webp" {
		ext = "webp"
	}

	part, err := writer.CreateFormFile("file", "combination."+ext)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+"/combinations/image", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("error subiendo imagen: %s", string(respBytes))
	}

	var result struct {
		CombinationID string `json:"combinationId"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", err
	}

	return result.CombinationID, nil
}

// CreateLock crea un nuevo lock con el combinationId dado
func (c *Client) CreateLock(combinationID string, durationSeconds int, isTest bool) (string, error) {
	payload := map[string]interface{}{
		"minDuration":          durationSeconds,
		"maxDuration":          durationSeconds,
		"maxLimitDuration":     nil,
		"minLimitDuration":     nil,
		"displayRemainingTime": true,
		"limitLockTime":        false,
		"combinationId":        combinationID,
		"extensions":           []interface{}{},
		"allowSessionOffer":    false,
		"hideTimeLogs":         false,
		"isTestLock":           isTest,
	}

	data, err := c.doRequest("POST", "/locks", payload)
	if err != nil {
		return "", err
	}

	var result struct {
		LockID string `json:"lockId"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	return result.LockID, nil
}

// UnlockLock desbloquea un lock para poder ver la combinación
func (c *Client) UnlockLock(lockID string) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/locks/%s/unlock", lockID), nil)
	return err
}

// ArchiveLock archiva un lock después de desbloquearlo
func (c *Client) ArchiveLock(lockID string) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/locks/%s/archive", lockID), nil)
	return err
}

// CombinationResult resultado del endpoint de combinación
type CombinationResult struct {
	Type         string `json:"type"`
	ImageFullURL string `json:"imageFullUrl"`
}

// GetCombination obtiene la combinación de un lock terminado
func (c *Client) GetCombination(lockID string) (*CombinationResult, error) {
	data, err := c.doRequest("GET", fmt.Sprintf("/locks/%s/combination", lockID), nil)
	if err != nil {
		return nil, err
	}

	var result CombinationResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DownloadCombinationImage descarga la imagen de combinación desde la URL firmada
func (c *Client) DownloadCombinationImage(imageURL string) ([]byte, error) {
	resp, err := c.httpClient.Get(imageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("error descargando imagen: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// FormatDuration formatea segundos en string legible
func FormatDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}
