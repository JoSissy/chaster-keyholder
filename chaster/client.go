package chaster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"chaster-keyholder/models"
)

const baseURL = "https://api.chaster.app"

// Client maneja el Public API de Chaster (token de usuario)
type Client struct {
	token      string
	httpClient *http.Client
	ext        *ExtensionClient // nil si no está configurado
}

// ExtensionClient maneja el Extensions API de Chaster (developer token de la extensión)
type ExtensionClient struct {
	token         string
	extensionSlug string
	httpClient    *http.Client
}

// NewClient crea un cliente para el Public API
func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// WithExtension añade soporte para el Extensions API.
// extensionToken: developer token de tu extensión en Chaster.
// extensionSlug: slug de tu extensión (ej: "jolie-keyholder").
func (c *Client) WithExtension(extensionToken, extensionSlug string) *Client {
	c.ext = &ExtensionClient{
		token:         extensionToken,
		extensionSlug: extensionSlug,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
	return c
}

// HasExtension indica si el cliente de extensión está configurado
func (c *Client) HasExtension() bool {
	return c.ext != nil && c.ext.token != "" && c.ext.extensionSlug != ""
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	return doHTTPRequest(c.httpClient, c.token, method, baseURL+path, body)
}

func (e *ExtensionClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	return doHTTPRequest(e.httpClient, e.token, method, baseURL+path, body)
}

// doHTTPRequest es la función base compartida por ambos clientes
func doHTTPRequest(httpClient *http.Client, token, method, url string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
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

// ── Locks ──────────────────────────────────────────────────────────────────

// GetActiveLock obtiene la sesión de castidad activa.
// Incluye locks "ready to unlock" para que el bot pueda detectar el fin de sesión.
func (c *Client) GetActiveLock() (*models.ChasterLock, error) {
	data, err := c.doRequest("GET", "/locks", nil)
	if err != nil {
		return nil, err
	}

	var locks []struct {
		models.ChasterLock
		CanBeUnlocked bool   `json:"canBeUnlocked"`
		StartDateRaw  string `json:"startDate"`
	}
	if err := json.Unmarshal(data, &locks); err != nil {
		return nil, err
	}

	for i := range locks {
		if locks[i].Status == "locked" {
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

// GetLockByID obtiene un lock específico por su ID, independientemente del estado.
// ErrLockNotFound se retorna cuando el lock no existe o ya fue desbloqueado/archivado.
var ErrLockNotFound = fmt.Errorf("lock not found or already unlocked")

// Usado para verificar si un lock específico terminó sin depender de GetActiveLock.
// Retorna ErrLockNotFound si el lock devuelve 404 (desbloqueado o archivado en Chaster).
func (c *Client) GetLockByID(lockID string) (*models.ChasterLock, error) {
	data, err := c.doRequest("GET", fmt.Sprintf("/locks/%s", lockID), nil)
	if err != nil {
		// 404 = lock desbloqueado o archivado — no es un error de red
		if strings.Contains(err.Error(), "404") {
			return nil, ErrLockNotFound
		}
		return nil, err
	}

	var lock struct {
		models.ChasterLock
		StartDateRaw string `json:"startDate"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, lock.StartDateRaw); err == nil {
			lock.ChasterLock.StartDate = t
			break
		}
	}

	return &lock.ChasterLock, nil
}

// ── Tiempo ─────────────────────────────────────────────────────────────────

// AddTime añade tiempo a la sesión (segundos positivos)
func (c *Client) AddTime(lockID string, seconds int) error {
	if seconds <= 0 {
		return fmt.Errorf("AddTime requiere segundos positivos, usa RemoveTime para quitar")
	}
	payload := map[string]int{"duration": seconds}
	_, err := c.doRequest("POST", fmt.Sprintf("/locks/%s/update-time", lockID), payload)
	return err
}

// RemoveTime quita tiempo a la sesión usando la Extensions API si está disponible,
// o el endpoint público con valor negativo como fallback.
func (c *Client) RemoveTime(lockID string, seconds int) error {
	if seconds <= 0 {
		return fmt.Errorf("RemoveTime requiere segundos positivos")
	}
	// Intentar via Extensions API primero (remove_time con valor positivo)
	if c.HasExtension() {
		sessionID, err := c.GetSessionByLockID(lockID)
		if err == nil {
			return c.doExtensionAction(sessionID, actionWithParams{
				Name:   "remove_time",
				Params: seconds,
			})
		}
	}
	// Fallback: endpoint público con valor negativo
	payload := map[string]int{"duration": -seconds}
	_, err := c.doRequest("POST", fmt.Sprintf("/locks/%s/update-time", lockID), payload)
	return err
}

// ── Acciones de extensión ──────────────────────────────────────────────────

type extensionAction struct {
	Action interface{} `json:"action"`
}

type actionSimple struct {
	Name string `json:"name"`
}

type actionWithParams struct {
	Name   string      `json:"name"`
	Params interface{} `json:"params"`
}

// ExtensionSession representa una sesión de extensión activa.
// _id es el ID de la extensión dentro del lock (no útil para acciones).
// sessionId es el ID real para el endpoint de acciones.
// lockId puede venir directo o anidado en lock._id.
type ExtensionSession struct {
	ID        string `json:"_id"`
	SessionID string `json:"sessionId"` // ← campo correcto para acciones
	LockID    string `json:"lockId"`
	Lock      *struct {
		ID string `json:"_id"`
	} `json:"lock"`
	Status string `json:"status"`
}

// resolvedLockID devuelve el lockId sin importar cómo lo devuelva la API
func (s ExtensionSession) resolvedLockID() string {
	if s.LockID != "" {
		return s.LockID
	}
	if s.Lock != nil {
		return s.Lock.ID
	}
	return ""
}

// resolvedSessionID devuelve el sessionId correcto para el endpoint de acciones
func (s ExtensionSession) resolvedSessionID() string {
	if s.SessionID != "" {
		return s.SessionID
	}
	return s.ID // fallback al _id si sessionId no existe
}

// GetSessionByLockID busca el sessionId de extensión correspondiente a un lockId.
func (c *Client) GetSessionByLockID(lockID string) (string, error) {
	if !c.HasExtension() {
		return "", fmt.Errorf("extensión no configurada: falta CHASTER_EXTENSION_TOKEN o CHASTER_EXTENSION_SLUG")
	}

	payload := map[string]interface{}{
		"status":           "locked",
		"extensionSlug":    c.ext.extensionSlug,
		"limit":            50,
		"paginationLastId": nil,
	}

	data, err := c.ext.doRequest("POST", "/api/extensions/sessions/search", payload)
	if err != nil {
		return "", fmt.Errorf("error buscando sesiones de extensión: %w", err)
	}

	// Loguear respuesta cruda completa para entender la estructura
	log.Printf("[GetSessionByLockID] respuesta cruda: %s", string(data))

	var result struct {
		Results []ExtensionSession `json:"results"`
		Count   int                `json:"count"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("error parseando sesiones: %w — respuesta: %s", err, string(data))
	}

	for _, s := range result.Results {
		log.Printf("[GetSessionByLockID] sesión: id=%s sessionId=%s lockId=%s", s.ID, s.SessionID, s.resolvedLockID())
		if s.resolvedLockID() == lockID {
			sessionID := s.resolvedSessionID()
			log.Printf("[GetSessionByLockID] match! usando sessionId=%s", sessionID)
			return sessionID, nil
		}
	}

	// Debug: listar qué lockIds encontramos para diagnosticar
	found := []string{}
	for _, s := range result.Results {
		found = append(found, s.resolvedLockID())
	}
	return "", fmt.Errorf("no se encontró sesión para lock %s (sesiones encontradas: %v, total: %d)", lockID, found, result.Count)
}

// doExtensionAction ejecuta una acción usando el token de extensión
func (c *Client) doExtensionAction(sessionID string, action interface{}) error {
	if !c.HasExtension() {
		return fmt.Errorf("extensión no configurada: falta CHASTER_EXTENSION_TOKEN o CHASTER_EXTENSION_SLUG")
	}
	payload := extensionAction{Action: action}
	_, err := c.ext.doRequest("POST", fmt.Sprintf("/api/extensions/sessions/%s/action", sessionID), payload)
	return err
}

// FreezeLock congela el lock dado su lockId (resuelve sessionId automáticamente)
func (c *Client) FreezeLock(lockID string) error {
	sessionID, err := c.GetSessionByLockID(lockID)
	if err != nil {
		return err
	}
	return c.doExtensionAction(sessionID, actionSimple{Name: "freeze"})
}

// UnfreezeLock descongela el lock dado su lockId
func (c *Client) UnfreezeLock(lockID string) error {
	sessionID, err := c.GetSessionByLockID(lockID)
	if err != nil {
		return err
	}
	return c.doExtensionAction(sessionID, actionSimple{Name: "unfreeze"})
}

// ToggleFreezeLock alterna congelación dado su lockId
func (c *Client) ToggleFreezeLock(lockID string) error {
	sessionID, err := c.GetSessionByLockID(lockID)
	if err != nil {
		return err
	}
	return c.doExtensionAction(sessionID, actionSimple{Name: "toggle_freeze"})
}

// SetTimerVisibility muestra u oculta el tiempo restante dado su lockId
func (c *Client) SetTimerVisibility(lockID string, visible bool) error {
	sessionID, err := c.GetSessionByLockID(lockID)
	if err != nil {
		return err
	}
	return c.doExtensionAction(sessionID, actionWithParams{
		Name:   "set_display_remaining_time",
		Params: visible,
	})
}

// PilloryParams parámetros para el cepo
type PilloryParams struct {
	Duration int    `json:"duration"` // segundos
	Reason   string `json:"reason,omitempty"`
}

// PutInPillory pone al portador en el cepo dado su lockId
func (c *Client) PutInPillory(lockID string, durationSeconds int, reason string) error {
	sessionID, err := c.GetSessionByLockID(lockID)
	if err != nil {
		return err
	}
	params := PilloryParams{Duration: durationSeconds, Reason: reason}
	return c.doExtensionAction(sessionID, actionWithParams{
		Name:   "pillory",
		Params: params,
	})
}

// ── Combinaciones e imágenes ───────────────────────────────────────────────

// UploadCombinationImage sube la foto del candado y devuelve el combinationId
func (c *Client) UploadCombinationImage(imageBytes []byte, mimeType string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

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

// CreateLock crea un nuevo lock con el combinationId dado.
// Incluye las extensiones necesarias: pillory y chaster-ai-1.
// durationSeconds se usa como min y max — la IA decide el valor exacto.
func (c *Client) CreateLock(combinationID string, durationSeconds int) (string, error) {
	extensions := []interface{}{
		map[string]interface{}{
			"slug": "pillory",
			"config": map[string]interface{}{
				"timeToAdd":          3600,
				"limitToLoggedUsers": true,
			},
			"mode":       "unlimited",
			"regularity": 3600,
		},
		map[string]interface{}{
			"slug": "tasks",
			"config": map[string]interface{}{
				"tasks": []interface{}{
					map[string]interface{}{
						"task":                 "Show your chastity cage is locked and worn",
						"points":               0,
						"verificationRequired": true,
						"duration":             1800,
					},
					map[string]interface{}{
						"task":                 "Insert your plug and show it in use",
						"points":               0,
						"verificationRequired": true,
						"duration":             1800,
					},
				},
				"voteEnabled":                    false,
				"voteDuration":                   1800, // 30 minutos en segundos
				"startVoteAfterLastVote":         false,
				"enablePoints":                   false,
				"pointsRequired":                 0,
				"allowWearerToEditTasks":         true,
				"allowWearerToConfigureTasks":    false,
				"preventWearerFromAssigningTasks": false,
				"allowWearerToChooseTasks":       false,
				"actionsOnAbandonedTask": []interface{}{
					map[string]interface{}{
						"name": "pillory",
						"params": map[string]interface{}{
							"duration": 900, // 15 min en cepo
						},
					},
				},
				"peerVerification": map[string]interface{}{
					"enabled": true,
					"punishments": []interface{}{
						map[string]interface{}{
							"name":   "add_time",
							"params": 3600, // +1h si la comunidad rechaza
						},
					},
				},
			},
			"mode":       "unlimited",
			"regularity": 3600,
		},
	}

	// Extensión verification-picture (verificación comunitaria de la jaula)
	extensions = append(extensions, map[string]interface{}{
		"slug": "verification-picture",
		"config": map[string]interface{}{
			"visibility": "all",
			"peerVerification": map[string]interface{}{
				"enabled": true,
				"punishments": []interface{}{
					map[string]interface{}{
						"name":   "pillory",
						"params": map[string]interface{}{"duration": 900},
					},
				},
			},
		},
		"mode":       "unlimited",
		"regularity": 86400,
	})

	// Añadir extensión de chaster-ai solo si está configurada
	if c.HasExtension() {
		extensions = append(extensions, map[string]interface{}{
			"slug":       c.ext.extensionSlug,
			"config":     map[string]interface{}{},
			"mode":       "unlimited",
			"regularity": 3600,
		})
	}

	payload := map[string]interface{}{
		"minDuration":          durationSeconds,
		"maxDuration":          durationSeconds,
		"maxLimitDuration":     nil,
		"minLimitDuration":     nil,
		"displayRemainingTime": true,
		"limitLockTime":        false,
		"combinationId":        combinationID,
		"extensions":           extensions,
		"allowSessionOffer":    false,
		"hideTimeLogs":         false,
		"isTestLock":           false,
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

// UnlockLock desbloquea un lock.
// Ignora error 400 solo si el mensaje indica que ya estaba desbloqueado.
func (c *Client) UnlockLock(lockID string) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/locks/%s/unlock", lockID), nil)
	if err != nil {
		// Si ya estaba desbloqueado no es un error real
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "400") {
			log.Printf("[UnlockLock] lock %s ya desbloqueado o no elegible: %v", lockID, err)
			return nil
		}
		return err
	}
	return nil
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

// ── Verification Picture ──────────────────────────────────────────────────

// VerificationPictureState estado de la extensión verification-picture del lock.
type VerificationPictureState struct {
	Code       string // currentVerificationCode
	HasPending bool   // true si hay una request activa esperando foto
}

// GetVerificationPictureState devuelve el estado actual de la extensión verification-picture.
// HasPending=true significa que ya hay una request activa — no se debe crear una nueva.
func (c *Client) GetVerificationPictureState(lockID string) (VerificationPictureState, error) {
	data, err := c.doRequest("GET", fmt.Sprintf("/locks/%s", lockID), nil)
	if err != nil {
		return VerificationPictureState{}, err
	}

	var lock struct {
		Extensions []struct {
			Slug     string `json:"slug"`
			UserData struct {
				CurrentVerificationCode string `json:"currentVerificationCode"`
				Verifications           []struct {
					Status string `json:"status"`
				} `json:"verifications"`
			} `json:"userData"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return VerificationPictureState{}, fmt.Errorf("error parseando lock: %w", err)
	}

	for _, ext := range lock.Extensions {
		if ext.Slug != "verification-picture" {
			continue
		}
		ud := ext.UserData
		hasPending := false
		for _, v := range ud.Verifications {
			if v.Status == "pending" {
				hasPending = true
				break
			}
		}
		return VerificationPictureState{
			Code:       ud.CurrentVerificationCode,
			HasPending: hasPending,
		}, nil
	}
	return VerificationPictureState{}, fmt.Errorf("extensión verification-picture no encontrada en el lock")
}

// RequestVerificationPicture solicita una nueva verificación de foto al portador.
func (c *Client) RequestVerificationPicture(lockID string) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/extensions/verification-picture/%s/request", lockID), nil)
	return err
}

// SubmitVerificationPicture sube la foto de verificación al endpoint de Chaster.
func (c *Client) SubmitVerificationPicture(lockID string, imageBytes []byte, mimeType string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	ext := "jpg"
	if mimeType == "image/png" {
		ext = "png"
	} else if mimeType == "image/webp" {
		ext = "webp"
	}

	part, err := writer.CreateFormFile("file", "checkin."+ext)
	if err != nil {
		return err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return err
	}
	if err := writer.WriteField("enableVerificationCode", "true"); err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+fmt.Sprintf("/extensions/verification-picture/%s/submit", lockID), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("error enviando verificación: %d %s", resp.StatusCode, string(respBytes))
	}
	return nil
}

// ── Tasks de extensión ────────────────────────────────────────────────────

// TaskHistoryEntry entrada en el historial de tareas de la extensión
type TaskHistoryEntry struct {
	Status string `json:"status"` // "verified"|"pending_verification"|"abandoned"|"rejected"|"completed"
	Task   struct {
		Task string `json:"task"`
	} `json:"task"`
	PeerVerification *struct {
		Status string `json:"status"` // "ongoing"|"verified"|"rejected"
	} `json:"peerVerification,omitempty"`
}

// AssignChasterTask asigna una tarea al portador usando la Extensions API.
// La tarea requiere verificación comunitaria.
func (c *Client) AssignChasterTask(sessionID, taskDescription string) error {
	if !c.HasExtension() {
		return fmt.Errorf("extensión no configurada: falta CHASTER_EXTENSION_TOKEN o CHASTER_EXTENSION_SLUG")
	}
	payload := map[string]interface{}{
		"task": map[string]interface{}{
			"task":                 taskDescription,
			"points":               1,
			"verificationRequired": true,
		},
		"actor": "extension",
	}
	_, err := c.ext.doRequest("POST", fmt.Sprintf("/api/extensions/sessions/%s/tasks/assign", sessionID), payload)
	return err
}

// UploadVerificationPhoto sube una foto al endpoint /files/upload y devuelve el verificationPictureToken.
func (c *Client) UploadVerificationPhoto(imageBytes []byte, mimeType string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	ext := "jpg"
	if mimeType == "image/png" {
		ext = "png"
	} else if mimeType == "image/webp" {
		ext = "webp"
	}

	part, err := writer.CreateFormFile("files", "verification."+ext)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", err
	}
	if err := writer.WriteField("type", "peer_verification"); err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+"/files/upload", &buf)
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
		return "", fmt.Errorf("error subiendo foto: %d %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("error parseando respuesta de upload: %w — body: %s", err, string(respBytes))
	}
	if result.Token == "" {
		return "", fmt.Errorf("token vacío en respuesta: %s", string(respBytes))
	}
	return result.Token, nil
}

// CompleteTaskWithVerification completa una tarea con foto de verificación usando el user token.
func (c *Client) CompleteTaskWithVerification(lockID, verificationPictureToken string) error {
	payload := map[string]interface{}{
		"isCompleted":              true,
		"verificationPictureToken": verificationPictureToken,
	}
	_, err := c.doRequest("POST", fmt.Sprintf("/extensions/tasks/%s/complete-task", lockID), payload)
	return err
}

// GetTaskHistory obtiene el historial de tareas de un lock.
func (c *Client) GetTaskHistory(lockID string) ([]TaskHistoryEntry, error) {
	data, err := c.doRequest("GET", fmt.Sprintf("/extensions/tasks/%s/history", lockID), nil)
	if err != nil {
		return nil, err
	}
	var entries []TaskHistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

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
