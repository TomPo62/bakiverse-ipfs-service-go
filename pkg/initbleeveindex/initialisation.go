package initbleeveindex
import (
	"database/sql"
	"log"

	"github.com/TomPo62/bakiverse-ipfs-service-go/internal/handlers"
	"github.com/blevesearch/bleve/v2"
)

func IndexInitialData(db *sql.DB, index bleve.Index) {
	rows, err := db.Query("SELECT cid, file_name, mime_type FROM files WHERE is_private = false")
	if err != nil {
			log.Fatal("Erreur lors de la récupération des fichiers :", err)
	}
	defer rows.Close()

	for rows.Next() {
			var doc handler.FileDoc
			if err := rows.Scan(&doc.CID, &doc.FileName, &doc.MimeType); err != nil {
					log.Println("Erreur lors de la lecture d'un fichier :", err)
					continue
			}
			doc.IsPrivate = false

			// Ajouter le fichier à l’index Bleve
			if err := index.Index(doc.CID, doc); err != nil {
					log.Println("Erreur d'indexation de", doc.CID, ":", err)
			}
	}
}
