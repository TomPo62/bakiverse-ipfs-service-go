package main

import (
	"log"
	"net/http"
	"os"

	"github.com/TomPo62/bakiverse-ipfs-service-go/internal/handlers"
	"github.com/TomPo62/bakiverse-ipfs-service-go/pkg/bleve"
	"github.com/TomPo62/bakiverse-ipfs-service-go/pkg/cors"
	"github.com/TomPo62/bakiverse-ipfs-service-go/pkg/database"
	"github.com/TomPo62/bakiverse-ipfs-service-go/pkg/initbleeveindex"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func main() {
	// Charger les variables d'environnement
	if err := godotenv.Load(); err != nil {
		log.Fatal("Erreur lors du chargement des variables d'environnement :", err)
	}

	// Connexion à la base de données
	db, err := database.ConnectDB()
	if err != nil {
		log.Fatal("Erreur lors de la connexion à la base de données :", err)
	}
	defer db.Close()

	index, err := bleve.InitBleveIndex()
	if err != nil {
		log.Fatal("Erreur lors de la création de l'index Bleve:", err)
	}
	defer index.Close()

	// Vérifier si l’indexation initiale doit être effectuée
	if os.Getenv("INIT_INDEX") == "true" {
		log.Println("Démarrage de l'indexation initiale...")
		initbleeveindex.IndexInitialData(db, index)
		log.Println("Indexation initiale terminée avec succès.")
	}

	h := &handler.Handler{
		DB:    db,
		Index: index,
	}

	// Configurer les routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, from Baki-IPFS-Service!"))
	})

	mux.HandleFunc("/upload", h.UploadFileHandler)

	mux.HandleFunc("/public-files", h.GetPublicFilesHandler)
	mux.HandleFunc("/private-files", h.GetAllFilesForAPIKeyHandler)

	mux.HandleFunc("/file", h.GetFileByCIDHandler)

	mux.HandleFunc("/file/display", h.DisplayFileByCIDHandler)

	mux.HandleFunc("/search-public-files", h.SearchPublicFilesHandler)

	mux.HandleFunc("/file/img", h.GetImageByCIDHandler)
	mux.HandleFunc("/file/private/img", h.GetPrivateImageByCIDHandler)

	mux.HandleFunc("/file/toggle-private", h.ToggleFilePrivacyHandler)


	mux.HandleFunc("/file/lottie", h.GetLottieFileByCIDHandler)

	mux.HandleFunc("/cid-themes", h.GetCidThemesHandler)
	mux.HandleFunc("/cid-themes/add", h.AddCidThemeHandler)
	mux.HandleFunc("/cid-themes/update", h.UpdateCidThemeHandler)
	mux.HandleFunc("/cid-themes/delete", h.DeleteCidThemeHandler)

	mux.HandleFunc("/docs/create", h.CreateDocHandler)
	mux.HandleFunc("/docs/get", h.GetDocHandler)
	mux.HandleFunc("/docs/update", h.UpdateDocHandler)
	mux.HandleFunc("/docs/delete", h.DeleteDocHandler)
	mux.HandleFunc("/docs/all", h.GetAllDocsHandler)

	// Configuration CORS
	handlerWithCORS := cors.CORSMiddleware(mux)

	// Démarrer le serveur
	log.Println("Serveur IPFS démarré sur le port 8085")
	log.Fatal(http.ListenAndServe(":8085", handlerWithCORS))
}
