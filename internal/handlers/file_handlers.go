package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TomPo62/bakiverse-ipfs-service-go/internal/service"
	"github.com/TomPo62/bakiverse-ipfs-service-go/pkg/security"

	bleve "github.com/blevesearch/bleve/v2"
)

type FileDoc struct {
	CID       string `json:"cid"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
	IsPrivate bool   `json:"is_private"`
}

// Handler struct to hold dependencies
type Handler struct {
	DB *sql.DB
	Index bleve.Index
}

// UploadFileHandler handles the file upload process
func (h *Handler) UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la requête est bien en méthode POST
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

	// Vérifier si l'API key existe et récupérer son ID
	var apiKeyID int
	err := h.DB.QueryRow("SELECT id FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID)
	if err != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Récupérer la valeur de is_private depuis le formulaire
	isPrivateStr := r.FormValue("is_private")
	isPrivate, err := strconv.ParseBool(isPrivateStr)
	if err != nil {
		http.Error(w, "Invalid is_private value", http.StatusBadRequest)
		return
	}

	// Récupérer le fichier depuis le formulaire
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Récupérer les métadonnées du fichier : nom, type MIME et taille
	fileName := header.Filename
	mimeType := header.Header.Get("Content-Type")
	fileSize := header.Size

	// Sauvegarder temporairement le fichier
	tempFilePath := filepath.Join("/tmp", filepath.Base(fileName))
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		http.Error(w, "Unable to create temp file", http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	// Copier le contenu du fichier uploadé vers le fichier temporaire
	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Uploader le fichier vers IPFS
	cid, err := service.UploadFileToIPFS(tempFilePath)
	if err != nil {
		http.Error(w, "Failed to upload to IPFS", http.StatusInternalServerError)
		return
	}

	// Supprimer le fichier temporaire
	os.Remove(tempFilePath)

	// Insérer les informations du fichier dans la base de données
	_, err = h.DB.Exec(
		"INSERT INTO files (api_key_id, cid, is_private, file_name, mime_type, file_size) VALUES (?, ?, ?, ?, ?, ?)",
		apiKeyID, cid, isPrivate, fileName, mimeType, fileSize,
	)
	if err != nil {
		http.Error(w, "Failed to save file metadata", http.StatusInternalServerError)
		return
	}

	if !isPrivate { // Seulement si le fichier est public
		doc := FileDoc{
				CID:       cid,
				FileName:  fileName,
				MimeType:  mimeType,
				IsPrivate: isPrivate,
		}
		err = h.Index.Index(cid, doc)
		if err != nil {
				http.Error(w, "Erreur d'indexation du fichier", http.StatusInternalServerError)
				return
		}
}

	// Réponse HTTP
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"cid":"%s", "message":"File uploaded successfully"}`, cid)))
}

// GetPublicFilesHandler handles fetching all public files
func (h *Handler) GetPublicFilesHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la requête est bien en méthode GET
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var err error

	// Récupérer l'API key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier si l'API key existe et récupérer son ID
	var apiKeyID int
	err = h.DB.QueryRow("SELECT id FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID)
	if err != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 10

	if pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
	}

	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			limit = 10
		}
	}

	offset := (page - 1) * limit

	var totalFiles int
	err = h.DB.QueryRow("SELECT COUNT(*) FROM files WHERE is_private = false").Scan(&totalFiles)
	if err != nil {
		http.Error(w, "Failed to query total files", http.StatusInternalServerError)
		return
	}

	// Récupérer tous les fichiers publics (is_private = false) depuis la base de données
	rows, err := h.DB.Query("SELECT cid, file_name, mime_type, file_size FROM files WHERE is_private = false LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		http.Error(w, "Failed to query public files", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Créer une slice pour stocker les informations des fichiers publics
	var publicFiles []map[string]interface{}

	// Itérer sur les résultats de la requête
	for rows.Next() {
		var cid, fileName, mimeType string
		var fileSize int64
		if err := rows.Scan(&cid, &fileName, &mimeType, &fileSize); err != nil {
			http.Error(w, "Failed to scan file metadata", http.StatusInternalServerError)
			return
		}
		publicFiles = append(publicFiles, map[string]interface{}{
			"cid":       cid,
			"file_name": fileName,
			"mime_type": mimeType,
			"file_size": fileSize,
		})
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Erreur lors de l'itération des résultats", http.StatusInternalServerError)
		return
	}

	// Construire la réponse avec les informations de pagination
	response := map[string]interface{}{
		"files":      publicFiles,
		"total":      totalFiles,
		"page":       page,
		"limit":      limit,
		"totalPages": (totalFiles + limit - 1) / limit,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Erreur lors de l'encodage de la réponse", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetFileByCIDHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling request to get file by CID")

	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	log.Println("Verifying API key:", apiKey)
	exists, apiErr := security.APIKeyExists(h.DB, apiKey)
	if apiErr != nil {
		log.Println("Error checking API key:", apiErr)
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	if !exists {
		log.Println("Invalid API Key provided")
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Récupérer le CID à partir des paramètres de l'URL
	cid := r.URL.Query().Get("cid")
	if cid == "" {
		log.Println("CID missing in request")
		http.Error(w, "CID manquant", http.StatusBadRequest)
		return
	}
	log.Println("Request for CID:", cid)

	// Vérifier si le fichier existe dans la base de données
	var fileName, mimeType string
	var fileSize int64
	err := h.DB.QueryRow("SELECT file_name, mime_type, file_size FROM files WHERE cid = ?", cid).Scan(&fileName, &mimeType, &fileSize)
	if err == sql.ErrNoRows {
		log.Println("File not found for CID:", cid)
		http.Error(w, "Fichier non trouvé", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println("Error fetching file metadata:", err)
		http.Error(w, "Erreur lors de la récupération des métadonnées", http.StatusInternalServerError)
		return
	}
	log.Println("Found file:", fileName, "of type:", mimeType, "and size:", fileSize)

	// Télécharger le contenu du fichier depuis IPFS
	content, err := service.DownloadFileFromIPFS(cid)
	if err != nil {
		log.Println("Error downloading file from IPFS:", err)
		http.Error(w, "Erreur lors de la récupération du fichier depuis IPFS", http.StatusInternalServerError)
		return
	}

	// Vérifier que le contenu n'est pas vide
	if len(content) == 0 {
		log.Println("File content is empty for CID:", cid)
		http.Error(w, "Contenu du fichier vide ou non récupéré", http.StatusInternalServerError)
		return
	}

	// Définir les en-têtes HTTP pour le type MIME et la taille du fichier
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", fileName)) // inline pour affichage direct
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))

	// Envoyer le contenu du fichier dans la réponse HTTP
	_, err = w.Write(content)
	if err != nil {
		log.Println("Error sending file content:", err)
		http.Error(w, "Erreur lors de l'envoi du fichier", http.StatusInternalServerError)
		return
	}

	log.Println("Successfully served file:", fileName)
}

func (h *Handler) GetImageByCIDHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling request to get image by CID")

	// Vérifier que la requête est bien en méthode GET
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var err error

	// Extraire le CID depuis l'URL
	cid := r.URL.Query().Get("cid")
	if cid == "" {
		http.Error(w, "CID manquant", http.StatusBadRequest)
		return
	}

	// Vérifier si le fichier est une image dans la base de données
	var fileName, mimeType string
	var fileSize int64
	err = h.DB.QueryRow("SELECT file_name, mime_type, file_size FROM files WHERE cid = ? AND is_private = false", cid).Scan(&fileName, &mimeType, &fileSize)
	if err == sql.ErrNoRows {
		http.Error(w, "Image non trouvée ou accès non autorisé", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Erreur lors de la récupération des métadonnées", http.StatusInternalServerError)
		return
	}

	// Vérifier que le type MIME est bien une image
	if !strings.HasPrefix(mimeType, "image/") {
		http.Error(w, "Le fichier demandé n'est pas une image", http.StatusBadRequest)
		return
	}

	// Récupérer le contenu de l'image depuis IPFS
	content, err := service.DownloadFileFromIPFS(cid)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération de l'image depuis IPFS", http.StatusInternalServerError)
		return
	}

	// Définir les en-têtes HTTP pour le type MIME et la taille du fichier
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))

	// Envoyer le contenu de l'image dans la réponse HTTP
	_, err = w.Write(content)
	if err != nil {
		http.Error(w, "Erreur lors de l'envoi de l'image", http.StatusInternalServerError)
		return
	}

	log.Println("Successfully served image:", fileName)
}

func (h *Handler) GetPrivateImageByCIDHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling request to get private image by CID")

	// Vérifier que la requête est bien en méthode GET
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Récupérer l'API key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier si l'API key existe et récupérer son ID
	var apiKeyID int
	err := h.DB.QueryRow("SELECT id FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID)
	if err != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Extraire le CID depuis l'URL
	cid := r.URL.Query().Get("cid")
	if cid == "" {
		http.Error(w, "CID manquant", http.StatusBadRequest)
		return
	}

	// Vérifier si le fichier est une image privée appartenant à l'utilisateur
	var fileName, mimeType string
	var fileSize int64
	var fileApiKeyID int
	err = h.DB.QueryRow("SELECT file_name, mime_type, file_size, api_key_id FROM files WHERE cid = ? AND is_private = true", cid).Scan(&fileName, &mimeType, &fileSize, &fileApiKeyID)
	if err == sql.ErrNoRows {
		http.Error(w, "Image non trouvée ou accès non autorisé", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Erreur lors de la récupération des métadonnées", http.StatusInternalServerError)
		return
	}

	// Vérifier que l'utilisateur est bien le propriétaire du fichier
	if apiKeyID != fileApiKeyID {
		http.Error(w, "Accès non autorisé", http.StatusUnauthorized)
		return
	}

	// Vérifier que le type MIME est bien une image
	if !strings.HasPrefix(mimeType, "image/") {
		http.Error(w, "Le fichier demandé n'est pas une image", http.StatusBadRequest)
		return
	}
	log.Println("Téléchargement du fichier depuis IPFS avec le CID:", cid)
	// Récupérer le contenu de l'image depuis IPFS
	content, err := service.DownloadFileFromIPFS(cid)
	if err != nil {
		log.Println("Erreur lors du téléchargement depuis IPFS:", err)
		http.Error(w, "Erreur lors de la récupération de l'image depuis IPFS", http.StatusInternalServerError)
		return
	}
	if len(content) == 0 {
		log.Println("Contenu du fichier IPFS vide ou non récupéré pour le CID:", cid)
		http.Error(w, "Contenu du fichier vide ou non récupéré", http.StatusInternalServerError)
		return
	}

	// Définir les en-têtes HTTP pour le type MIME et la taille du fichier
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))

	// Envoyer le contenu de l'image dans la réponse HTTP
	_, err = w.Write(content)
	if err != nil {
		http.Error(w, "Erreur lors de l'envoi de l'image", http.StatusInternalServerError)
		return
	}

	log.Println("Successfully served private image:", fileName)
}

func (h *Handler) GetAllFilesForAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la requête est bien en méthode GET
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Récupérer l'API key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier si l'API key existe et récupérer son ID
	var apiKeyID int
	err := h.DB.QueryRow("SELECT id FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID)
	if err != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Récuperer les parametres de pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	page := 1
	limit := 10
	if pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
	}

	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			limit = 10
		}
	}

	offset := (page - 1) * limit

	// Compter le nombre total de fichiers privés
	var totalFiles int
	err = h.DB.QueryRow("SELECT COUNT(*) FROM files WHERE api_key_id = ?", apiKeyID).Scan(&totalFiles)
	if err != nil {
		http.Error(w, "Erreur lors du comptage des fichiers", http.StatusInternalServerError)
		return
	}

	// Récupérer tous les fichiers associés à l'API Key
	rows, err := h.DB.Query("SELECT cid, is_private, file_name, mime_type, file_size FROM files WHERE api_key_id = ? LIMIT ? OFFSET ?", apiKeyID, limit, offset)
	if err != nil {
		http.Error(w, "Failed to query private files", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Créer une slice pour stocker les informations des fichiers privés
	var privateFiles []map[string]interface{}

	// Itérer sur les résultats de la requête
	for rows.Next() {
		var cid, fileName, mimeType string
		var isPrivate bool
		var fileSize int64
		if err := rows.Scan(&cid, &isPrivate, &fileName, &mimeType, &fileSize); err != nil {
			http.Error(w, "Failed to scan file metadata", http.StatusInternalServerError)
			return
		}
		privateFile := map[string]interface{}{
			"cid":       cid,
			"is_private": isPrivate,
			"file_name": fileName,
			"mime_type": mimeType,
			"file_size": fileSize,
		}
		privateFiles = append(privateFiles, privateFile)
	}

	// Vérifier les erreurs après l'itération
	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterating over rows", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"files":      privateFiles,
		"total":      totalFiles,
		"page":       page,
		"limit":      limit,
		"totalPages": (totalFiles + limit - 1) / limit,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Erreur lors de l'encodage de la réponse", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetLottieFileByCIDHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling request to get Lottie file by CID")

	// Vérifier que la requête est bien en méthode GET
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Récupérer l'API key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	// Vérifier si l'API key existe
	exists, err := security.APIKeyExists(h.DB, apiKey)
	if err != nil || !exists {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// Extraire le CID depuis l'URL
	cid := r.URL.Query().Get("cid")
	if cid == "" {
		http.Error(w, "CID manquant", http.StatusBadRequest)
		return
	}

	// Vérifier si le fichier est un Lottie file dans la base de données
	var fileName, mimeType string
	var fileSize int64
	err = h.DB.QueryRow("SELECT file_name, mime_type, file_size FROM files WHERE cid = ? AND is_private = false", cid).Scan(&fileName, &mimeType, &fileSize)
	if err == sql.ErrNoRows {
		http.Error(w, "Lottie file non trouvé ou accès non autorisé", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Erreur lors de la récupération des métadonnées", http.StatusInternalServerError)
		return
	}

	// Vérifier que le type MIME est bien 'application/json'
	if mimeType != "application/json" {
		http.Error(w, "Le fichier demandé n'est pas un Lottie file", http.StatusBadRequest)
		return
	}

	// Récupérer le contenu du fichier depuis IPFS
	content, err := service.DownloadFileFromIPFS(cid)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du fichier depuis IPFS", http.StatusInternalServerError)
		return
	}

	// Définir les en-têtes HTTP pour le type MIME et la taille du fichier
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache pendant 1 jour

	// Envoyer le contenu du fichier dans la réponse HTTP
	_, err = w.Write(content)
	if err != nil {
		http.Error(w, "Erreur lors de l'envoi du fichier", http.StatusInternalServerError)
		return
	}

	log.Println("Successfully served Lottie file:", fileName)
}

func (h *Handler) DisplayFileByCIDHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling request to display file by CID")

	// Vérification de la méthode de requête
	if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
	}

	// Récupération du CID à partir des paramètres de l'URL
	cid := r.URL.Query().Get("cid")
	if cid == "" {
			http.Error(w, "CID manquant", http.StatusBadRequest)
			return
	}

	// Récupération des métadonnées du fichier avec vérification de is_private = false
	var fileName, mimeType string
	var fileSize int64
	err := h.DB.QueryRow(
			"SELECT file_name, mime_type, file_size FROM files WHERE cid = ? AND is_private = false",
			cid,
	).Scan(&fileName, &mimeType, &fileSize)

	if err == sql.ErrNoRows {
			http.Error(w, "Fichier non trouvé ou accès non autorisé", http.StatusNotFound)
			return
	} else if err != nil {
			http.Error(w, "Erreur lors de la récupération des métadonnées", http.StatusInternalServerError)
			return
	}

	// Téléchargement du contenu du fichier depuis IPFS
	content, err := service.DownloadFileFromIPFS(cid)
	if err != nil {
			http.Error(w, "Erreur lors de la récupération du fichier depuis IPFS", http.StatusInternalServerError)
			return
	}

	// Vérification que le contenu n'est pas vide
	if len(content) == 0 {
			http.Error(w, "Contenu du fichier vide ou non récupéré", http.StatusInternalServerError)
			return
	}

	// Définition des en-têtes HTTP pour afficher le fichier directement dans le navigateur
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", "inline; filename="+fileName)
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))

	// Écriture du contenu du fichier dans la réponse HTTP
	_, err = w.Write(content)
	if err != nil {
			log.Println("Erreur lors de l'envoi du fichier :", err)
			http.Error(w, "Erreur lors de l'envoi du fichier", http.StatusInternalServerError)
			return
	}

	log.Printf("Fichier %s servi avec succès pour affichage", fileName)
}

func (h *Handler) SearchPublicFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Missing search query", http.StatusBadRequest)
		return
	}

	// Récupérer les paramètres de pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	page := 1
	limit := 10

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Calculer l'offset pour la pagination
	from := (page - 1) * limit

	// Configurer la requête de recherche avec la pagination
	searchRequest := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(query), limit, from, false)
	searchRequest.Fields = []string{"cid", "file_name", "mime_type"} // Champs à récupérer

	searchResult, err := h.Index.Search(searchRequest)
	if err != nil {
		http.Error(w, "Erreur lors de la recherche", http.StatusInternalServerError)
		return
	}

	// Construire la réponse avec les résultats paginés
	results := []map[string]interface{}{}
	for _, hit := range searchResult.Hits {
		result := map[string]interface{}{
			"cid":       hit.Fields["cid"],
			"file_name": hit.Fields["file_name"],
			"mime_type": hit.Fields["mime_type"],
		}
		results = append(results, result)
	}

	// Envoyer la réponse en JSON
	response := map[string]interface{}{
		"results":    results,
		"total":      searchResult.Total,
		"totalPages": (int64(searchResult.Total) + int64(limit) - 1) / int64(limit), // Nombre total de pages
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Erreur lors de l'encodage de la réponse", http.StatusInternalServerError)
	}
}

func (h *Handler) ToggleFilePrivacyHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la requête est bien en méthode POST
	if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
	}

	// Récupérer l'API Key depuis les headers
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
			http.Error(w, "Missing API Key", http.StatusUnauthorized)
			return
	}

	// Vérifier si l'API Key existe et récupérer son ID
	var apiKeyID int
	err := h.DB.QueryRow("SELECT id FROM api_keys WHERE api_key = ?", apiKey).Scan(&apiKeyID)
	if err != nil {
			http.Error(w, "Invalid API Key", http.StatusUnauthorized)
			return
	}

	// Récupérer le CID du fichier dans les paramètres de la requête
	cid := r.URL.Query().Get("cid")
	if cid == "" {
			http.Error(w, "Missing CID", http.StatusBadRequest)
			return
	}

	// Vérifier que le fichier est associé à l'API Key
	var isPrivate bool
	err = h.DB.QueryRow("SELECT is_private FROM files WHERE cid = ? AND api_key_id = ?", cid, apiKeyID).Scan(&isPrivate)
	if err == sql.ErrNoRows {
			http.Error(w, "File not found or unauthorized access", http.StatusNotFound)
			return
	} else if err != nil {
			http.Error(w, "Error retrieving file information", http.StatusInternalServerError)
			return
	}

	// Toggle de `is_private` et mise à jour dans la base de données
	newPrivacyStatus := !isPrivate
	_, err = h.DB.Exec("UPDATE files SET is_private = ? WHERE cid = ? AND api_key_id = ?", newPrivacyStatus, cid, apiKeyID)
	if err != nil {
			http.Error(w, "Failed to update file privacy status", http.StatusInternalServerError)
			return
	}

	// Réponse en JSON
	response := map[string]interface{}{
			"cid":       cid,
			"is_private": newPrivacyStatus,
			"message":   "File privacy status updated successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}


