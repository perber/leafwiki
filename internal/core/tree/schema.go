package tree

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

const CurrentSchemaVersion = 2

type SchemaInfo struct {
	Version int `json:"version"`
}

func loadSchema(storageDir string) (SchemaInfo, error) {
	path := filepath.Join(storageDir, "schema.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// First run / legacy install
			log.Printf("Schema file not found, assuming version 0")
			return SchemaInfo{Version: 0}, nil
		}
		log.Printf("Error reading schema file: %v", err)
		return SchemaInfo{}, err
	}

	var s SchemaInfo
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("Error unmarshaling schema file: %v", err)
		return SchemaInfo{}, err
	}

	return s, nil
}

func saveSchema(storageDir string, version int) error {
	path := filepath.Join(storageDir, "schema.json")
	s := SchemaInfo{Version: version}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Printf("Error marshaling schema data: %v", err)
		return err
	}

	log.Printf("Saving schema version %d to %s", version, path)
	return os.WriteFile(path, data, 0o644)
}
