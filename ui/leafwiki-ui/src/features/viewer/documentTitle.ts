// documentTitle is the browser title of a page view: "<title> - <site name>".
// The server renders the same format into the shell of public pages
// (seo.DocumentTitle in internal/wiki/seo/page_meta.go); change both together
// so the title doesn't change once the app has loaded.
export function documentTitle(
  title: string | undefined,
  siteName: string,
): string {
  return title ? `${title} - ${siteName}` : siteName
}
