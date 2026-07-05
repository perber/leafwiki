import { uploadAsset } from '@/lib/api/assets'

// Matches markdown image references whose source is an inline base64 data URI,
// e.g. from `![alt](data:image/png;base64,AAAA...)`.
const DATA_URI_IMAGE_RE =
  /!\[([^\]]*)\]\(data:([a-zA-Z0-9.+-]+\/[a-zA-Z0-9.+-]+);base64,([A-Za-z0-9+/=]+)\)/g

function estimateBase64DecodedSize(base64: string): number {
  const padding = base64.endsWith('==') ? 2 : base64.endsWith('=') ? 1 : 0
  return Math.floor((base64.length * 3) / 4) - padding
}

function dataUriToFile(mimeType: string, base64: string, name: string): File {
  const byteString = atob(base64)
  const bytes = new Uint8Array(byteString.length)
  for (let i = 0; i < byteString.length; i++) {
    bytes[i] = byteString.charCodeAt(i)
  }
  return new File([bytes], name, { type: mimeType })
}

/**
 * Rich-pasted HTML (Word/Outlook/some web pages) can embed images as inline
 * base64 data URIs rather than referencing an uploaded file. Turndown copies
 * those verbatim into the markdown, which would bypass the asset-upload
 * pipeline and bloat the page's stored content by the full image payload.
 * This uploads each inline image as a real asset and rewrites the reference
 * to the returned URL; images that fail to upload or exceed the size limit
 * are dropped (kept as alt text only) rather than left inlined.
 */
export async function uploadInlineDataUriImages(
  markdown: string,
  pageId: string,
  maxAssetUploadSizeBytes: number,
): Promise<string> {
  const matches = Array.from(markdown.matchAll(DATA_URI_IMAGE_RE))
  if (matches.length === 0) return markdown

  let result = markdown
  let index = 0
  for (const match of matches) {
    const [fullMatch, alt, mimeType, base64] = match
    index += 1
    try {
      if (!mimeType.startsWith('image/')) {
        result = result.replace(fullMatch, alt)
        continue
      }
      if (estimateBase64DecodedSize(base64) > maxAssetUploadSizeBytes) {
        result = result.replace(fullMatch, alt)
        continue
      }
      const ext = mimeType.split('/')[1]?.split('+')[0] || 'png'
      const file = dataUriToFile(
        mimeType,
        base64,
        `pasted-image-${Date.now()}-${index}.${ext}`,
      )
      if (file.size > maxAssetUploadSizeBytes) {
        result = result.replace(fullMatch, alt)
        continue
      }
      const res = await uploadAsset(pageId, file)
      result = result.replace(fullMatch, `![${alt}](${res.file})`)
    } catch {
      result = result.replace(fullMatch, alt)
    }
  }
  return result
}
