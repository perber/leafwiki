// Cross-browser phrasings a native `import()` rejects with when the fetched
// chunk 404s — e.g. a stale hash left over from before a redeploy. Detecting
// these lets a UI offer "reload the page" instead of a dead error, since a
// reload re-fetches the current build's asset references and self-heals.
const DYNAMIC_IMPORT_ERROR_PATTERNS = [
  /failed to fetch dynamically imported module/i,
  /error loading dynamically imported module/i,
  /importing a module script failed/i,
]

export function isDynamicImportChunkError(message: string): boolean {
  return DYNAMIC_IMPORT_ERROR_PATTERNS.some((pattern) => pattern.test(message))
}
