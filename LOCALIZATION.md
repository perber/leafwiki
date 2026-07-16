# LeafWiki localization

LeafWiki UI strings live in JSON files under `locales/`. **You do not need to rebuild the app** to add or update a language — place the files next to the binary and reload the page in the browser.

## Directory layout

```
leafwiki
locales/
  languages.json      # display names for the language switcher
  en/
    common.json
    auth.json
    ...
  de/                 # example: community translation
    common.json
    ...
```

By default the server looks for `locales/` **next to the `data/` directory**:

```
my-wiki/
  leafwiki
  data/
  locales/
    en/
    de/
```

Override the path with `--locales-dir` or the `LEAFWIKI_LOCALES_DIR` environment variable.

## Adding a new language

1. Copy `locales/en/` to `locales/<code>/` (for example `locales/de/`).
2. Translate the **values** in each JSON file. **Do not change keys.**
3. Register the language in `locales/languages.json`:

```json
{
  "en": "English",
  "de": "Deutsch"
}
```

4. Restart LeafWiki if it is already running (or reload the browser after the files are in place).
5. Pick the language in the UI (language icon in the top bar) or set it manually:

```javascript
localStorage.setItem('leafwiki-lang', 'de')
location.reload()
```

## Translation files (namespaces)

| File | Contents |
|------|----------|
| `common.json` | Shared buttons, errors, pagination |
| `auth.json` | Login, user menu |
| `sidebar.json` | Sidebar panels |
| `tree.json` | Page tree |
| `page.json` | Page create, delete, move dialogs |
| `editor.json` | Markdown editor |
| `viewer.json` | Page viewer |
| `search.json` | Search |
| `history.json` | Revision history |
| `users.json` | User management |
| `assets.json` | Media library |
| `branding.json` | Branding settings |
| `backup.json` | Git backup |
| `importer.json` | Import tool |
| `maintenance.json` | Maintenance |
| `errors.json` | API error messages |

## Rules for translators

- Use plain JSON, UTF-8 encoding (no BOM).
- Keep placeholders such as `{{version}}` and `{{name}}` unchanged.
- Do not remove HTML tags inside strings when present.
- Validate JSON before submitting.
- English (`en/`) is the reference. Missing keys fall back to English bundled in the frontend build.

## Development vs deployment

Source locale files for the UI live in `ui/leafwiki-ui/src/locales/`.

The server serves copies from the repository-root `locales/` folder. After editing files under `src/locales/`, sync them to the root:

```bash
cp -r ui/leafwiki-ui/src/locales/en locales/en
cp -r ui/leafwiki-ui/src/locales/<code> locales/<code>
```

## Changing language in the UI

1. Click the **language** icon in the top-right of the header.
2. Select a language from the list.
3. The choice is stored in the browser (`localStorage` key: `leafwiki-lang`).

On first visit, the language is chosen from the browser setting (`navigator.language`), with English as the default fallback.

## HTTP API

- `GET /api/locales` — list available languages
- `GET /locales/{lang}/{namespace}.json` — translation file

Example: `GET /locales/en/common.json`
