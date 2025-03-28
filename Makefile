# Name der Binary
BINARY_NAME=leafwiki

# Speicherort der main.go
CMD_DIR=./cmd/leafwiki

# Default Ziel: Build
all: build

# Build der Binary
build:
	go build -o $(BINARY_NAME) $(CMD_DIR)

# Ausführen der App
run:
	go run $(CMD_DIR)

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)

# Go-Tests für alle Pakete
test:
	go test ./...


# Hilfe anzeigen
help:
	@echo "Makefile Befehle:"
	@echo "  make build     – Baut die LeafWiki Binary"
	@echo "  make run       – Führt das Projekt aus"
	@echo "  make test      – Führt alle Tests aus"
	@echo "  make clean     – Löscht die Binary"

.PHONY: all build run clean test fmt lint help