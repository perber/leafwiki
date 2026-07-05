import { describe, expect, it } from 'vitest'
import { replaceFilenameInText } from './editorCommands'

describe('replaceFilenameInText', () => {
  it('updates both the src filename and the alt text when the alt text matches the old filename', () => {
    const doc = '![old-name.png](/assets/old-name.png)'
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe('![new-name.png](/assets/new-name.png)')
  })

  it('updates the src filename but preserves custom alt text that does not match the old filename', () => {
    const doc = '![a nice photo](/assets/old-name.png)'
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe('![a nice photo](/assets/new-name.png)')
  })

  it('updates plain (non-image) markdown links the same way', () => {
    const doc = '[old-name.pdf](/assets/old-name.pdf)'
    const result = replaceFilenameInText(doc, 'old-name.pdf', 'new-name.pdf')
    expect(result).toBe('[new-name.pdf](/assets/new-name.pdf)')
  })

  it('handles filenames containing regex-special characters', () => {
    const doc = '![old (1).png](/assets/old (1).png)'
    const result = replaceFilenameInText(doc, 'old (1).png', 'new.png')
    expect(result).toBe('![new.png](/assets/new.png)')
  })

  it('replaces every occurrence in the document', () => {
    const doc = [
      '![old-name.png](/assets/old-name.png)',
      'See also [old-name.png](/assets/old-name.png) below.',
    ].join('\n')
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe(
      [
        '![new-name.png](/assets/new-name.png)',
        'See also [new-name.png](/assets/new-name.png) below.',
      ].join('\n'),
    )
  })

  it('does not match across multiple links on the same line', () => {
    const doc =
      '![foo.png](/assets/foo.png) and ![old-name.png](/assets/old-name.png)'
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe(
      '![foo.png](/assets/foo.png) and ![new-name.png](/assets/new-name.png)',
    )
  })
})
