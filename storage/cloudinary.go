package storage

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CloudinaryClient struct {
	cloudName  string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
}

type CloudinaryUploadResult struct {
	SecureURL string `json:"secure_url"`
	PublicID  string `json:"public_id"`
}

func NewCloudinaryClient(cloudName, apiKey, apiSecret string) *CloudinaryClient {
	return &CloudinaryClient{
		cloudName:  cloudName,
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Upload sube una imagen a Cloudinary y devuelve (secureURL, publicID, error)
func (c *CloudinaryClient) Upload(imageBytes []byte, mimeType, folder string) (string, string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Generar firma
	params := map[string]string{
		"folder":    folder,
		"timestamp": timestamp,
	}
	signature := c.generateSignature(params)

	// Construir multipart
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	ext := "jpg"
	if mimeType == "image/png" {
		ext = "png"
	} else if mimeType == "image/webp" {
		ext = "webp"
	}

	part, err := writer.CreateFormFile("file", "upload."+ext)
	if err != nil {
		return "", "", err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", "", err
	}

	writer.WriteField("api_key", c.apiKey)
	writer.WriteField("timestamp", timestamp)
	writer.WriteField("signature", signature)
	writer.WriteField("folder", folder)
	writer.Close()

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", c.cloudName)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("cloudinary error %d: %s", resp.StatusCode, string(body))
	}

	var result CloudinaryUploadResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", err
	}

	return result.SecureURL, result.PublicID, nil
}

// Delete elimina una imagen de Cloudinary por su public_id
func (c *CloudinaryClient) Delete(publicID string) error {
	if publicID == "" {
		return nil
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{
		"public_id": publicID,
		"timestamp": timestamp,
	}
	signature := c.generateSignature(params)

	formData := fmt.Sprintf("public_id=%s&timestamp=%s&api_key=%s&signature=%s",
		publicID, timestamp, c.apiKey, signature)

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/destroy", c.cloudName)
	req, err := http.NewRequest("POST", url, strings.NewReader(formData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudinary delete error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// generateSignature genera la firma SHA1 requerida por Cloudinary
func (c *CloudinaryClient) generateSignature(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := []string{}
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}

	toSign := strings.Join(parts, "&") + c.apiSecret
	h := sha1.New()
	h.Write([]byte(toSign))
	return fmt.Sprintf("%x", h.Sum(nil))
}
