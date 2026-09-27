// "On this page" only makes sense once there's something to jump to; a
// page with attachments but no headings should label the button/panel after
// what it actually lists instead.
export function tocTitleKey(
  entryCount: number,
  downloadCount: number,
): 'toc.onThisPage' | 'toc.downloads' {
  if (entryCount > 0) return 'toc.onThisPage'
  if (downloadCount > 0) return 'toc.downloads'
  return 'toc.onThisPage'
}
