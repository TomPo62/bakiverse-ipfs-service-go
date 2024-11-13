package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type Doc struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Path        string  `json:"path"`
	DocSrc      string  `json:"doc_src"`
	Version     float64 `json:"version"`
	IsChildren  bool    `json:"is_children"`
	ParentID    *int     `json:"parent_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// checkAPIKeyWritePermission vérifie si l'API key est valide et possède les permissions de write
func checkAPIKeyWritePermission(db *sql.DB, apiKey string) (bool, error) {
	var permissions string
	err := db.QueryRow("SELECT permissions FROM api_keys WHERE api_key = ?", apiKey).Scan(&permissions)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("API key non trouvée dans la base de données.")
			return false, nil
		}
		log.Println("Erreur lors de la vérification des permissions de l'API key:", err)
		return false, err
	}
	log.Println("Permissions de l'API key:", permissions)
	return permissions == "write", nil
}

// CreateDocHandler gère la création d'un document
func (h *Handler) CreateDocHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	log.Println("API key reçue:", apiKey)
	hasPermission, err := checkAPIKeyWritePermission(h.DB, apiKey)
	if err != nil {
		log.Println("Erreur lors de la vérification de l'API key:", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if !hasPermission {
		log.Println("Permissions insuffisantes pour l'API key:", apiKey)
		http.Error(w, "API key non autorisée ou permissions insuffisantes", http.StatusUnauthorized)
		return
	}

	var doc Doc
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		log.Println("Erreur lors de la décodage du document JSON:", err)
		http.Error(w, "Erreur lors de la décodage du document", http.StatusBadRequest)
		return
	}
	log.Println("Données du document à créer:", doc)

	// Vérification du ParentID si non-nul
	if doc.ParentID != nil && *doc.ParentID != 0 {
		var parentExists bool
		err = h.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM docs WHERE id = ?)", *doc.ParentID).Scan(&parentExists)
		if err != nil {
			log.Println("Erreur lors de la vérification du ParentID:", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}
		if !parentExists {
			log.Println("ParentID invalide fourni:", *doc.ParentID)
			http.Error(w, "ParentID invalide", http.StatusBadRequest)
			return
		}
	} else {
		doc.ParentID = nil // Assurer que ParentID est NULL si non spécifié
	}

	result, err := h.DB.Exec("INSERT INTO docs (title, path, doc_src, version, is_children, parent_id) VALUES (?, ?, ?, ?, ?, ?)",
		doc.Title, doc.Path, doc.DocSrc, doc.Version, doc.IsChildren, doc.ParentID)
	if err != nil {
		log.Println("Erreur lors de l'insertion du document dans la base de données:", err)
		http.Error(w, "Erreur lors de la création du document", http.StatusInternalServerError)
		return
	}

	docID, err := result.LastInsertId()
	if err != nil {
		log.Println("Erreur lors de la récupération de l'ID du document créé:", err)
		http.Error(w, "Erreur lors de la récupération de l'ID du document", http.StatusInternalServerError)
		return
	}
	doc.ID = int(docID)

	log.Println("Document créé avec succès, ID:", doc.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

// GetDocHandler gère la récupération d'un document spécifique
func (h *Handler) GetDocHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	docIDStr := r.URL.Query().Get("id")
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		http.Error(w, "ID de document invalide", http.StatusBadRequest)
		return
	}

	var doc Doc
	err = h.DB.QueryRow("SELECT id, title, path, doc_src, version, is_children, parent_id, created_at, updated_at FROM docs WHERE id = ?", docID).Scan(
		&doc.ID, &doc.Title, &doc.Path, &doc.DocSrc, &doc.Version, &doc.IsChildren, &doc.ParentID, &doc.CreatedAt, &doc.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Document non trouvé", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Erreur lors de la récupération du document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

// UpdateDocHandler gère la mise à jour d'un document
func (h *Handler) UpdateDocHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	hasPermission, err := checkAPIKeyWritePermission(h.DB, apiKey)
	if err != nil || !hasPermission {
		http.Error(w, "API key non autorisée ou permissions insuffisantes", http.StatusUnauthorized)
		return
	}

	var doc Doc
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		http.Error(w, "Erreur lors de la décodage du document", http.StatusBadRequest)
		return
	}

	_, err = h.DB.Exec("UPDATE docs SET title = ?, path = ?, doc_src = ?, version = ?, is_children = ?, parent_id = ? WHERE id = ?",
		doc.Title, doc.Path, doc.DocSrc, doc.Version, doc.IsChildren, doc.ParentID, doc.ID)
	if err != nil {
		http.Error(w, "Erreur lors de la mise à jour du document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Document mis à jour avec succès"}`))
}

// DeleteDocHandler gère la suppression d'un document
func (h *Handler) DeleteDocHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	hasPermission, err := checkAPIKeyWritePermission(h.DB, apiKey)
	if err != nil || !hasPermission {
		http.Error(w, "API key non autorisée ou permissions insuffisantes", http.StatusUnauthorized)
		return
	}

	docIDStr := r.URL.Query().Get("id")
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		http.Error(w, "ID de document invalide", http.StatusBadRequest)
		return
	}

	_, err = h.DB.Exec("DELETE FROM docs WHERE id = ?", docID)
	if err != nil {
		http.Error(w, "Erreur lors de la suppression du document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Document supprimé avec succès"}`))
}

// GetAllDocsHandler gère la récupération de tous les documents
func (h *Handler) GetAllDocsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	rows, err := h.DB.Query("SELECT id, title, path, doc_src, version, is_children, parent_id, created_at, updated_at FROM docs")
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des documents", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var docs []Doc
	for rows.Next() {
		var doc Doc
		err := rows.Scan(&doc.ID, &doc.Title, &doc.Path, &doc.DocSrc, &doc.Version, &doc.IsChildren, &doc.ParentID, &doc.CreatedAt, &doc.UpdatedAt)
		if err != nil {
			http.Error(w, "Erreur lors de la lecture des données du document", http.StatusInternalServerError)
			return
		}
		docs = append(docs, doc)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Erreur lors de l'itération des documents", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(docs)
}
