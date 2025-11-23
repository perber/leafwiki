package tree

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type FrontMatter struct {
	Title     string    `yaml:"title"`
	ID        string    `yaml:"id"`
	Slug      string    `yaml:"slug"`
	Position  int       `yaml:"position"`
	CreatedAt time.Time `yaml:"created"` // Key-Mapping
	UpdatedAt time.Time `yaml:"updated"`
}

func NewFrontMatter(title, id, slug string, position int, createdAt time.Time, updatedAt time.Time) *FrontMatter {
	return &FrontMatter{
		Title:     title,
		ID:        id,
		Slug:      slug,
		Position:  position,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func (f *FrontMatter) Write() []byte {
	f.CreatedAt = f.CreatedAt.UTC()
	f.UpdatedAt = f.UpdatedAt.UTC()

	b, err := yaml.Marshal(f)
	if err != nil {
		fmt.Printf("frontmatter marshal error: %v\n", err)
		return []byte("---\n" + string(b) + "---\n\n")
	}
	return []byte("---\n" + string(b) + "---\n\n")
}

func ParseFrontMatter(doc []byte) (FrontMatter, []byte, bool) {
	var fm FrontMatter
	if !(bytes.HasPrefix(doc, []byte("---\n")) || bytes.HasPrefix(doc, []byte("---\r\n"))) {
		return fm, doc, false
	}

	// ab Start nach der ersten Zeile iterieren, bis eine Zeile EXAKT "---" ist
	i := 4 // len("---\n")
	if bytes.HasPrefix(doc, []byte("---\r\n")) {
		i = 5
	}

	// suche Zeilenende und prüfe auf "---"
	end := -1
	for off := i; off < len(doc); {
		// finde nächste Zeile
		nl := bytes.IndexByte(doc[off:], '\n')
		if nl < 0 {
			break
		}
		line := bytes.TrimRight(doc[off:off+nl], "\r\n")
		if bytes.Equal(line, []byte("---")) {
			end = off // Position des Zeilenanfangs mit '---'
			i = off + nl + 1
			break
		}
		off += nl + 1
	}
	if end < 0 {
		return fm, doc, false
	}

	raw := doc[4:end] // bei CRLF wäre 5 statt 4—vereinfachend könntest du oben startIdx berechnen
	if bytes.HasPrefix(doc, []byte("---\r\n")) {
		raw = doc[5:end]
	}
	body := doc[i:]
	_ = yaml.Unmarshal(raw, &fm)
	return fm, body, true
}

// Für Eingaben aus dem Editor: FM entfernen, falls der Editor es mitgeschickt hat
func StripFrontMatter(doc []byte) []byte {
	_, body, ok := ParseFrontMatter(doc)
	if !ok {
		return doc
	}

	// remove newline at teh beginning of the body if it exists
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	return body
}

// UpdateFrontMatterOnFile liest filePath, parsed FM (oder erzeugt einen),
// ruft mutate(fm) auf, setzt UpdatedAt (wenn setUpdated==true) und schreibt FM+Body zurück.
func UpdateFrontMatterOnFile(
	filePath string,
	setUpdated bool,
	mutate func(*FrontMatter) error,
) error {
	orig, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	fm, body, ok := ParseFrontMatter(orig)
	if !ok {
		// Fallback: created aus mtime, falls möglich
		created := time.Now().UTC()
		if info, err := os.Stat(filePath); err == nil {
			created = info.ModTime().UTC()
		}
		fm = FrontMatter{
			CreatedAt: created,
			UpdatedAt: created,
		}
		// Body ist kompletter Inhalt
		body = orig
	}

	if mutate != nil {
		if err := mutate(&fm); err != nil {
			return err
		}
	}
	if setUpdated {
		fm.UpdatedAt = time.Now().UTC()
	}

	// schreiben (truncate)
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(fm.Write()); err != nil {
		return fmt.Errorf("write fm: %w", err)
	}
	if _, err := f.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}

// WriteFrontMatterAndBody ersetzt den gesamten Inhalt mit fm+body (z. B. bei Create).
func WriteFrontMatterAndBody(filePath string, fm *FrontMatter, body []byte) error {
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(fm.Write()); err != nil {
		return fmt.Errorf("write fm: %w", err)
	}
	if _, err := f.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}
