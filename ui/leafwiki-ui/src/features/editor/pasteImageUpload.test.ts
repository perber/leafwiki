import { beforeEach, describe, expect, it, vi } from 'vitest'

const uploadAssetMock = vi.fn()
vi.mock('@/lib/api/assets', () => ({
  uploadAsset: (...args: unknown[]) => uploadAssetMock(...args),
}))

import { uploadInlineDataUriImages } from './pasteImageUpload'

// A valid 1x1 transparent PNG, base64-encoded.
const TINY_PNG_BASE64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

describe('uploadInlineDataUriImages', () => {
  beforeEach(() => {
    uploadAssetMock.mockReset()
  })

  it('returns markdown unchanged when there are no data URI images', async () => {
    const markdown = 'Hello **world**\n\n![alt](/assets/existing.png)'
    const result = await uploadInlineDataUriImages(
      markdown,
      'page-1',
      1_000_000,
    )
    expect(result).toBe(markdown)
    expect(uploadAssetMock).not.toHaveBeenCalled()
  })

  it('uploads an inline base64 image and rewrites the reference to the returned URL', async () => {
    uploadAssetMock.mockResolvedValue({ file: '/assets/page-1/uploaded.png' })
    const markdown = `![a picture](data:image/png;base64,${TINY_PNG_BASE64})`

    const result = await uploadInlineDataUriImages(
      markdown,
      'page-1',
      1_000_000,
    )

    expect(result).toBe('![a picture](/assets/page-1/uploaded.png)')
    expect(uploadAssetMock).toHaveBeenCalledTimes(1)
    const [pageId, file] = uploadAssetMock.mock.calls[0]
    expect(pageId).toBe('page-1')
    expect(file).toBeInstanceOf(File)
    expect(file.type).toBe('image/png')
  })

  it('drops the image (keeps alt text) when it exceeds the size limit', async () => {
    const markdown = `![big](data:image/png;base64,${TINY_PNG_BASE64})`

    const result = await uploadInlineDataUriImages(markdown, 'page-1', 1)

    expect(result).toBe('big')
    expect(uploadAssetMock).not.toHaveBeenCalled()
  })

  it('drops a non-image data URI instead of uploading it', async () => {
    const markdown = '![not image](data:text/plain;base64,SGVsbG8=)'

    const result = await uploadInlineDataUriImages(
      markdown,
      'page-1',
      1_000_000,
    )

    expect(result).toBe('not image')
    expect(uploadAssetMock).not.toHaveBeenCalled()
  })

  it('drops the image (keeps alt text) when the upload fails', async () => {
    uploadAssetMock.mockRejectedValue(new Error('network error'))
    const markdown = `![broken](data:image/png;base64,${TINY_PNG_BASE64})`

    const result = await uploadInlineDataUriImages(
      markdown,
      'page-1',
      1_000_000,
    )

    expect(result).toBe('broken')
  })

  it('uploads multiple inline images independently', async () => {
    uploadAssetMock
      .mockResolvedValueOnce({ file: '/assets/page-1/one.png' })
      .mockResolvedValueOnce({ file: '/assets/page-1/two.png' })
    const markdown = `![one](data:image/png;base64,${TINY_PNG_BASE64})\n\n![two](data:image/png;base64,${TINY_PNG_BASE64})`

    const result = await uploadInlineDataUriImages(
      markdown,
      'page-1',
      1_000_000,
    )

    expect(result).toBe(
      '![one](/assets/page-1/one.png)\n\n![two](/assets/page-1/two.png)',
    )
    expect(uploadAssetMock).toHaveBeenCalledTimes(2)
  })
})
