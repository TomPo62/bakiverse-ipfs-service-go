package database

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"fmt"
	"log"
)

func ConnectDB() (*sql.DB, error) {
	// Charger les infos de connexion depuis les variables d'environnement
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbname := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbname)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
			log.Fatal("Failed to connect to the database:", err)
			return nil, err
	}
	if err := db.Ping(); err != nil {
			log.Fatal("Database ping failed:", err)
			return nil, err
	}
	log.Println("Connected to the database successfully")
	return db, nil
}
