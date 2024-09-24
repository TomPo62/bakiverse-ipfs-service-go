package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/TomPo62/bakiverse-ipfs-service-go/pkg/cors"
	"github.com/TomPo62/bakiverse-ipfs-service-go/pkg/database"

	"github.com/joho/godotenv" // Pour charger les variables d'environnement
)

var db *sql.DB // Déclaration de la variable globale

func main() {
	// Charger les variables d'environnement
	if err := godotenv.Load(); err != nil {
		log.Fatal("Erreur lors du chargement des variables d'environnement :", err)
	}

	// Connexion à la base de données MariaDB avec des variables d'env
	var err error                  // Déclare la variable err ici
	db, err = database.ConnectDB() // Affecte à la variable globale db
	if err != nil {
		log.Fatal("Erreur lors de la connexion à la base de données :", err)
	}
	defer db.Close()

	// Appliquer le middleware CORS à toutes les routes
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, from Baki-IPFS-Service!"))
	})

	finalMux := http.NewServeMux()
	finalMux.Handle("/", mux)
	handlerWithCORS := cors.CORSMiddleware(finalMux)

	// Démarrer le serveur HTTP
	log.Println("Serveur démarré sur le port 8085")
	log.Fatal(http.ListenAndServe(":8085", handlerWithCORS))
}
