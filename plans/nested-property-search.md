# Plan: Nested / Range Property Search

## Status: Bereit zur Umsetzung (post v0.11.0)

## Problem-Analyse

Drei Lücken mit unterschiedlichen Lösungsansätzen:

| Use Case          | Beispiel                    | Status              |
|-------------------|-----------------------------|---------------------|
| Scalar Exact-Match | `status = published`       | ✅ vorhanden        |
| List-Containment  | `owners enthält alice`      | ❌ fehlt            |
| Range-Query       | `priority > 5`              | ❌ fehlt            |
| Object-Field      | `meta.author = alice`       | ✅ via Dot-Notation |

## Schritt 1 — Schema-Erweiterung: `list_key`-Spalte

Beim Indexieren von Listen wird ein zusätzlicher Eintrag ohne Index-Suffix geschrieben.
PRIMARY KEY bleibt auf `(page_id, key)`, kein Konflikt. `list_key` ist nullable.

```
key=owners.0, value=alice, type=text, list_key=owners
key=owners.1, value=bob,   type=text, list_key=owners
```

Migration: `ALTER TABLE page_properties ADD COLUMN list_key TEXT`
Index: `CREATE INDEX idx_page_properties_list_key_value ON page_properties(list_key, value)`

Query für Containment:
```sql
SELECT DISTINCT page_id FROM page_properties
WHERE list_key = ? AND value = ?
```

## Schritt 2 — Schema-Erweiterung: `numeric_value`-Spalte

Für Range-Queries wird der numerische Wert in einer eigenen REAL-Spalte gespeichert.

```
key=priority, value="42", type=number, numeric_value=42.0
```

Migration: `ALTER TABLE page_properties ADD COLUMN numeric_value REAL`
Index: `CREATE INDEX idx_page_properties_key_numeric ON page_properties(key, numeric_value)`

Query für Range:
```sql
SELECT page_id FROM page_properties
WHERE key = ? AND numeric_value > ?
```

## Schritt 3 — Neue Store-Methoden

```go
// List-Containment: "owners enthält alice"
GetPageIDsByPropertyContains(listKey, value string) ([]string, error)

// Range: "priority > 5"  — op: "gt", "gte", "lt", "lte", "eq"
GetPageIDsByPropertyRange(key string, op string, threshold float64) ([]string, error)
```

## Schritt 4 — Service-Layer: PropertyFilter

```go
type PropertyFilter struct {
    Key      string
    Operator string  // "eq", "contains", "gt", "gte", "lt", "lte"
    Value    string
}

func (s *PropertiesService) FilterPages(filters []PropertyFilter) ([]string, error)
```

Mehrere Filter werden per INTERSECT oder in Go verknüpft (AND-Semantik).

## Schritt 5 — Index-Schreiblogik anpassen

In `extractFlatEntryDepth` (properties_service.go):
- Bei `[]interface{}`: `list_key = parent-prefix` in PropertyEntry setzen
- Bei `float64`/`int`: `numeric_value` in PropertyEntry füllen
- `PropertyEntry.Type`-Kommentar `// currently always "text"` entfernen

`PropertyEntry` wird um zwei optionale Felder erweitert:
```go
type PropertyEntry struct {
    Value        string
    Type         string
    ListKey      string   // gesetzt wenn dieser Eintrag ein Listen-Element ist
    NumericValue *float64 // gesetzt für type=number
}
```

## Schritt 6 — HTTP-API (separater Schritt, nach Bedarf)

```
GET /api/pages?filter[owners][contains]=alice&filter[priority][gt]=5
```

## Nicht im Scope

- Object-Containment (`meta` enthält irgendwo Wert `alice`) — kein realer Use Case, weglassen.
- Full-Text-Search auf Property-Values — dafür gibt es den Search-Index.
- OR-Verknüpfung mehrerer Filter — erstmal nur AND.
