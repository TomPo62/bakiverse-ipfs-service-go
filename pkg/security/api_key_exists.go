package security

import (
	"database/sql"
	"fmt"
)

func APIKeyExists(db *sql.DB, apiKey string) (bool, error) {
	var count int
	query := "SELECT COUNT(1) FROM api_keys WHERE api_key = ?"

	err := db.QueryRow(query, apiKey).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error checking API key: %v", err)
	}

	// Si count est supérieur à 0, l'API key existe
	return count > 0, nil
}
