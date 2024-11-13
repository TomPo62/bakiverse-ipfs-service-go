package cors

import (
	"net/http"
)

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Définir les en-têtes CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Disposition, Content-Length, Authorization, X-Api-Key, X-API-KEY")

		// Vérification des requêtes OPTIONS (pré-vol)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Passer à l'étape suivante si ce n'est pas une requête OPTIONS
		next.ServeHTTP(w, r)
	})
}
