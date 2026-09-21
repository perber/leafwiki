// Whether the TOC panel/dropdown should be shown for a page: normally only
// once it has more than 3 headings, but always (as soon as it has any
// heading at all) when the instance-wide "always show TOC" setting is on.
export function shouldShowToc(
  entryCount: number,
  alwaysShow: boolean,
): boolean {
  return entryCount > (alwaysShow ? 0 : 3)
}
