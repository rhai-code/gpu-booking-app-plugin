package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
)

func ListLLMEndpoints(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	db := database.DB()

	var rows *sql.Rows
	var err error

	if user.IsAdmin {
		rows, err = db.Query("SELECT "+database.LLMEndpointColumns+" FROM llm_endpoints ORDER BY id")
	} else {
		rows, err = db.Query(
			"SELECT "+database.LLMEndpointColumns+" FROM llm_endpoints WHERE is_global = 1 OR owner = ? ORDER BY id",
			user.Username,
		)
	}
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		log.Printf("error querying llm_endpoints: %v", err)
		return
	}
	defer rows.Close()

	endpoints := []database.LLMEndpoint{}
	for rows.Next() {
		e, err := database.ScanLLMEndpoint(rows)
		if err != nil {
			log.Printf("error scanning llm_endpoint: %v", err)
			continue
		}
		if e.APIKey != "" {
			e.APIKey = "****"
		}
		endpoints = append(endpoints, e)
	}

	JsonResponse(w, map[string]any{"endpoints": endpoints})
}

func GetLLMEndpoint(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	db := database.DB()

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		HttpError(w, http.StatusBadRequest, "invalid_id")
		return
	}

	var e database.LLMEndpoint
	var isGlobal, enabled int
	err = db.QueryRow(
		"SELECT "+database.LLMEndpointColumns+" FROM llm_endpoints WHERE id = ?", id,
	).Scan(&e.ID, &e.Name, &e.URL, &e.APIKey, &e.ModelName, &e.ProviderType, &e.Owner, &isGlobal, &enabled, &e.CreatedAt)
	if err == sql.ErrNoRows {
		HttpError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}
	e.IsGlobal = isGlobal == 1
	e.Enabled = enabled == 1

	if !user.IsAdmin && e.Owner != user.Username && !e.IsGlobal {
		HttpError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Internal endpoint fetch (from agent) gets the real key; browser gets masked
	if r.Header.Get("X-Internal-Request") != "true" {
		if e.APIKey != "" {
			e.APIKey = "****"
		}
	}

	JsonResponse(w, e)
}

func CreateLLMEndpoint(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	db := database.DB()

	var req struct {
		Name         string `json:"name"`
		URL          string `json:"url"`
		APIKey       string `json:"api_key"`
		ModelName    string `json:"model_name"`
		ProviderType string `json:"provider_type"`
		IsGlobal     bool   `json:"is_global"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HttpError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		HttpError(w, http.StatusBadRequest, "name_required")
		return
	}
	if req.ModelName == "" {
		HttpError(w, http.StatusBadRequest, "model_name_required")
		return
	}
	if req.ProviderType == "" {
		req.ProviderType = "openai-compatible"
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	isGlobal := 0
	if req.IsGlobal {
		isGlobal = 1
	}

	result, err := db.Exec(
		"INSERT INTO llm_endpoints (name, url, api_key, model_name, provider_type, owner, is_global, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)",
		req.Name, req.URL, req.APIKey, req.ModelName, req.ProviderType, user.Username, isGlobal, createdAt,
	)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		log.Printf("error creating llm_endpoint: %v", err)
		return
	}

	id, _ := result.LastInsertId()
	endpoint := database.LLMEndpoint{
		ID:           int(id),
		Name:         req.Name,
		URL:          req.URL,
		APIKey:       "****",
		ModelName:    req.ModelName,
		ProviderType: req.ProviderType,
		Owner:        user.Username,
		IsGlobal:     req.IsGlobal,
		Enabled:      true,
		CreatedAt:    createdAt,
	}
	if req.APIKey == "" {
		endpoint.APIKey = ""
	}

	JsonResponseStatus(w, http.StatusCreated, endpoint)
}

func UpdateLLMEndpoint(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	db := database.DB()

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		HttpError(w, http.StatusBadRequest, "invalid_id")
		return
	}

	var owner string
	err = db.QueryRow("SELECT owner FROM llm_endpoints WHERE id = ?", id).Scan(&owner)
	if err == sql.ErrNoRows {
		HttpError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}

	if owner != user.Username && !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		Name         string  `json:"name"`
		URL          string  `json:"url"`
		APIKey       *string `json:"api_key"`
		ModelName    string  `json:"model_name"`
		ProviderType string  `json:"provider_type"`
		IsGlobal     bool    `json:"is_global"`
		Enabled      bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HttpError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	isGlobal := 0
	if req.IsGlobal {
		isGlobal = 1
	}
	enabled := 0
	if req.Enabled {
		enabled = 1
	}

	if req.APIKey != nil && *req.APIKey != "****" {
		_, err = db.Exec(
			"UPDATE llm_endpoints SET name=?, url=?, api_key=?, model_name=?, provider_type=?, is_global=?, enabled=? WHERE id=?",
			req.Name, req.URL, *req.APIKey, req.ModelName, req.ProviderType, isGlobal, enabled, id,
		)
	} else {
		_, err = db.Exec(
			"UPDATE llm_endpoints SET name=?, url=?, model_name=?, provider_type=?, is_global=?, enabled=? WHERE id=?",
			req.Name, req.URL, req.ModelName, req.ProviderType, isGlobal, enabled, id,
		)
	}
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		log.Printf("error updating llm_endpoint %d: %v", id, err)
		return
	}

	JsonResponse(w, map[string]string{"status": "updated"})
}

func DeleteLLMEndpoint(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	db := database.DB()

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		HttpError(w, http.StatusBadRequest, "invalid_id")
		return
	}

	var owner string
	err = db.QueryRow("SELECT owner FROM llm_endpoints WHERE id = ?", id).Scan(&owner)
	if err == sql.ErrNoRows {
		HttpError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}

	if owner != user.Username && !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "forbidden")
		return
	}

	_, err = db.Exec("DELETE FROM llm_endpoints WHERE id = ?", id)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}

	JsonResponse(w, map[string]string{"status": "deleted"})
}

func TestLLMEndpoint(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	db := database.DB()

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		HttpError(w, http.StatusBadRequest, "invalid_id")
		return
	}

	var e database.LLMEndpoint
	var isGlobal, enabled int
	err = db.QueryRow(
		"SELECT "+database.LLMEndpointColumns+" FROM llm_endpoints WHERE id = ?", id,
	).Scan(&e.ID, &e.Name, &e.URL, &e.APIKey, &e.ModelName, &e.ProviderType, &e.Owner, &isGlobal, &enabled, &e.CreatedAt)
	if err == sql.ErrNoRows {
		HttpError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}
	e.IsGlobal = isGlobal == 1
	e.Enabled = enabled == 1

	if !user.IsAdmin && e.Owner != user.Username && !e.IsGlobal {
		HttpError(w, http.StatusForbidden, "forbidden")
		return
	}

	testURL := e.URL
	if e.ProviderType == "gemini" {
		testURL = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s", e.ModelName)
		if e.APIKey != "" {
			testURL += "?key=" + e.APIKey
		}
	} else if testURL != "" {
		if !strings.HasSuffix(testURL, "/") {
			testURL += "/"
		}
		testURL += "models"
	}

	if testURL == "" {
		JsonResponse(w, map[string]any{"status": "ok", "message": "No URL to test (Gemini uses API key only)"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", testURL, nil)
	if e.APIKey != "" && e.ProviderType != "gemini" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		JsonResponse(w, map[string]any{"status": "error", "message": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		JsonResponse(w, map[string]any{"status": "ok", "http_status": resp.StatusCode})
	} else {
		JsonResponse(w, map[string]any{"status": "error", "http_status": resp.StatusCode, "message": resp.Status})
	}
}
