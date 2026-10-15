// Desktop page views always reserve the right-hand TOC column, even when the
// page has too few headings (or none) for a contentful TOC.
//
// Navigating between a TOC page and a plain page used to mount/unmount
// TocSidePanel and toggle .page-viewer__subheader--toc-reserved. Chromium
// kept breadcrumbs aligned through that transition (#1420); Safari did not,
// and only reflowed after an unrelated hover (#1549 / #1378). Keeping the
// column reserved removes the transition entirely and matches the requested
// "always show On this page on desktop" behaviour.
export function shouldReserveTocPane(): boolean {
  return true
}
