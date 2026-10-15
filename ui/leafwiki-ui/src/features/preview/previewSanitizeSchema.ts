import { defaultSchema } from 'rehype-sanitize'

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
    '*': [
      ...(defaultSchema.attributes?.['*'] || []),
      'class',
      'className',
      'data-leafwiki-generated-id',
      'data-line',
      'data-line-numbers',
      'style',
    ],
    code: [
      ...(defaultSchema.attributes?.code || []),
      'className',
      'class',
      'data-line',
      'data-line-numbers',
    ],
    audio: [...(defaultSchema.attributes?.audio || []), 'controls', 'src'],
    video: [
      ...(defaultSchema.attributes?.video || []),
      'controls',
      'src',
      'preload',
    ],
  },
}
