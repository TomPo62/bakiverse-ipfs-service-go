package handler

import (
	"encoding/json"
	"net/http"
)

// CidTheme represents a theme with a CID and a name
type CidTheme struct {
	ID   int    `json:"id"`
	CID  string `json:"cid"`
	Name string `json:"name"`
}

// GetCidThemesHandler handles the GET request to retrieve all cidThemes
func (h *Handler) GetCidThemesHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la requête est bien en méthode GET
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Récupérer tous les cidThemes depuis la base de données
	rows, err := h.DB.Query("SELECT id, cid, name FROM cid_themes")
	if err != nil {
		http.Error(w, "Failed to query cidThemes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var cidThemes []CidTheme

	for rows.Next() {
		var theme CidTheme
		if err := rows.Scan(&theme.ID, &theme.CID, &theme.Name); err != nil {
			http.Error(w, "Failed to scan cidTheme", http.StatusInternalServerError)
			return
		}
		cidThemes = append(cidThemes, theme)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cidThemes); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// AddCidThemeHandler handles the POST request to add a new cidTheme
func (h *Handler) AddCidThemeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Récupérer l'API key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier si l'API key existe et récupérer l'ID et les permissions
	var apiKeyID int
	var permission string
	err := h.DB.QueryRow("SELECT id, permissions FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID, &permission)
	if err != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier que la permission est bien "write"
	if permission != "write" {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}


	var theme CidTheme
	if err := json.NewDecoder(r.Body).Decode(&theme); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Insérer le nouveau cidTheme dans la base de données
	_, err = h.DB.Exec("INSERT INTO cid_themes (cid, name) VALUES (?, ?)", theme.CID, theme.Name)
	if err != nil {
		http.Error(w, "Failed to insert cidTheme", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("CidTheme added successfully"))
}

func (h *Handler) UpdateCidThemeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Récupérer l'API key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier si l'API key existe et récupérer l'ID et les permissions
	var apiKeyID int
	var permission string
	err := h.DB.QueryRow("SELECT id, permissions FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID, &permission)
	if err != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier que la permission est bien "write"
	if permission != "write" {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	var theme CidTheme
	if err := json.NewDecoder(r.Body).Decode(&theme); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Mettre à jour le cidTheme dans la base de données
	_, err = h.DB.Exec("UPDATE cid_themes SET cid = ?, name = ? WHERE id = ?", theme.CID, theme.Name, theme.ID)
	if err != nil {
		http.Error(w, "Failed to update cidTheme", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("CidTheme updated successfully"))
}


// DeleteCidThemeHandler handles the DELETE request to delete an existing cidTheme
func (h *Handler) DeleteCidThemeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Récupérer l'API key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier si l'API key existe et récupérer l'ID et les permissions
	var apiKeyID int
	var permission string
	err := h.DB.QueryRow("SELECT id, permissions FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID, &permission)
	if err != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier que la permission est bien "write"
	if permission != "write" {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	// Récupérer l'ID du cidTheme depuis l'URL
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing ID parameter", http.StatusBadRequest)
		return
	}

	// Supprimer le cidTheme de la base de données
	_, err = h.DB.Exec("DELETE FROM cid_themes WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Failed to delete cidTheme", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("CidTheme deleted successfully"))
}
