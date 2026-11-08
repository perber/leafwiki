import { defaultSchema } from 'rehype-sanitize'
import { COLLAPSIBLE_CLASS } from './normalizeMarkdownBlocks'

// Sanitizer schema for rendered markdown: the default GitHub schema plus the
// wikilink protocols, media tags and the attributes LeafWiki's own plugins add.
// Only MarkdownPreview passes it to rehype-sanitize. Never mutate it: its nested
// arrays are shared with rehype-sanitize's defaultSchema.
export const previewSanitizeSchema = {
  ...defaultSchema,
  clobberPrefix: '',
  protocols: {
    ...defaultSchema.protocols,
    href: [
      ...(defaultSchema.protocols?.href ?? []),
      'wikilink-notfound',
      'wikilink-ambiguous',
    ],
    src: [...(defaultSchema.protocols?.src ?? []), 'http', 'https'],
  },
  tagNames: [...(defaultSchema.tagNames || []), 'audio', 'video', 'mark'],
  attributes: {
    ...defaultSchema.attributes,
    // No free class names: the page shares the app's stylesheet, so a class
    // such as `fixed inset-0 z-50` would let content cover the app. Classes
    // are allowed per element, only for names the markdown plugins and
    // normalizeMarkdownBlocks emit before this pass (the default schema
    // covers task lists and footnotes).
    //
    // No `name` on img: the browser exposes an img as a property of
    // `document` under its name, and under its id once it has a name, so
    // `<img name="cookie">` or `<img id="cookie" name="x">` would replace
    // document.cookie. img is the only allowed tag with that behaviour (form,
    // iframe, embed and object are not allowed), so `name` leaves '*' and
    // comes back on a, details and input. Ids stay unprefixed (clobberPrefix
    // '') so heading and footnote anchors keep their `#intro` form (#869).
    '*': [
      ...(defaultSchema.attributes?.['*'] || []).filter(
        (attribute) => attribute !== 'name',
      ),
      'data-leafwiki-generated-id',
      'data-line',
      'data-line-numbers',
      'style',
    ],
    a: [...(defaultSchema.attributes?.a || []), 'name'],
    input: [...(defaultSchema.attributes?.input || []), 'name'],
    code: [
      ...(defaultSchema.attributes?.code || []),
      'data-line',
      'data-line-numbers',
    ],
    // remark-flexible-markers: `flexible-marker`, `-default`, `-empty` and
    // one per marker color.
    mark: [['className', /^flexible-marker(?:-[a-z]+)?$/]],
    // normalizeMarkdownBlocks: `:::collapsible` / `:::collapsed`.
    details: [['className', COLLAPSIBLE_CLASS], 'name'],
    audio: [...(defaultSchema.attributes?.audio || []), 'controls', 'src'],
    video: [
      ...(defaultSchema.attributes?.video || []),
      'controls',
      'src',
      'preload',
    ],
  },
}
