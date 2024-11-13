package service

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/ipfs/go-ipfs-api"
)

func UploadFileToIPFS(filePath string) (string, error) {
	sh := shell.NewShell("localhost:5001") // Connecter à l'API d'IPFS

	// Vérifier que IPFS est bien accessible
	if !sh.IsUp() {
		return "", fmt.Errorf("IPFS node is not available")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Ajouter le fichier à IPFS
	cid, err := sh.Add(file)
	if err != nil {
		return "", err
	}

	return cid, nil
}

func DownloadFileFromIPFS(cid string) ([]byte, error) {
	sh := shell.NewShell("localhost:5001") // Connexion à l'API d'IPFS

	// Vérifier que le nœud IPFS est bien disponible
	if !sh.IsUp() {
		return nil, fmt.Errorf("IPFS node is not available")
	}

	// Télécharger le fichier depuis IPFS en utilisant le CID avec une tentative de répétition
	var buf bytes.Buffer
	for attempt := 1; attempt <= 3; attempt++ {
		log.Printf("Attempt %d: downloading file from IPFS, CID: %s\n", attempt, cid)
		readCloser, err := sh.Cat(cid)
		if err != nil {
			log.Printf("Failed to download file on attempt %d: %v\n", attempt, err)
			if attempt == 3 {
				return nil, fmt.Errorf("failed to download file from IPFS after multiple attempts: %v", err)
			}
			time.Sleep(2 * time.Second) // Attendre avant une nouvelle tentative
			continue
		}
		defer readCloser.Close()

		_, err = io.Copy(&buf, readCloser)
		if err != nil {
			log.Printf("Failed to read file content on attempt %d: %v\n", attempt, err)
			if attempt == 3 {
				return nil, fmt.Errorf("failed to read file content after multiple attempts: %v", err)
			}
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("Successfully downloaded file from IPFS, size: %d bytes\n", buf.Len())
		return buf.Bytes(), nil
	}

	// Si toutes les tentatives échouent
	return nil, fmt.Errorf("IPFS download attempts failed for CID: %s", cid)
}

