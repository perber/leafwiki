import { normalizeMarkdownBlocks } from './normalizeMarkdownBlocks'

describe('normalizeMarkdownBlocks', () => {
  describe('collapsible blocks', () => {
    it('renders a :::collapsible block as an open <details> with a summary', () => {
      const input = ':::collapsible Details\nHidden content here.\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '<details class="markdown-collapsible" open>\n' +
          '<summary>Details</summary>\n' +
          '\n' +
          'Hidden content here.\n' +
          '</details>',
      )
    })

    it('renders a :::collapsed block as a closed <details> without the open attribute', () => {
      const input = ':::collapsed\nBody line\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '<details class="markdown-collapsible">\nBody line\n</details>',
      )
    })

    it('omits the <summary> when no title is given', () => {
      const input = ':::collapsible\nBody\n:::'

      expect(normalizeMarkdownBlocks(input)).not.toContain('<summary>')
    })

    it('trims surrounding whitespace from the title', () => {
      const input = ':::collapsible    My Title   \nBody\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '<details class="markdown-collapsible" open>\n' +
          '<summary>My Title</summary>\n' +
          '\n' +
          'Body\n' +
          '</details>',
      )
    })

    it('renders an empty collapsible block with no body lines', () => {
      const input = ':::collapsible\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '<details class="markdown-collapsible" open>\n</details>',
      )
    })
  })

  describe('shoutout blocks', () => {
    it.each([
      ['caution', 'WARNING'],
      ['danger', 'ERROR'],
      ['error', 'ERROR'],
      ['fail', 'ERROR'],
      ['failed', 'ERROR'],
      ['failure', 'ERROR'],
      ['info', 'INFO'],
      ['note', 'INFO'],
      ['success', 'SUCCESS'],
      ['tip', 'INFO'],
      ['warn', 'WARNING'],
      ['warning', 'WARNING'],
    ])(
      'maps :::%s to a GitHub-style [!%s] quote block',
      (rawType, expectedLabel) => {
        const input = `:::${rawType}\nWatch out!\n:::`

        expect(normalizeMarkdownBlocks(input)).toBe(
          `> [!${expectedLabel}]\n>\n> Watch out!`,
        )
      },
    )

    it('passes unrecognized types through uppercased instead of mapping them', () => {
      const input = ':::custom\nBody\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe('> [!CUSTOM]\n>\n> Body')
    })

    it('normalizes underscores to hyphens in unrecognized types', () => {
      const input = ':::my_type\nBody\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe('> [!MY-TYPE]\n>\n> Body')
    })

    it('quotes every body line', () => {
      const input = ':::info\nLine one\nLine two\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '> [!INFO]\n>\n> Line one\n> Line two',
      )
    })

    it('renders an empty shoutout block with no body lines', () => {
      const input = ':::info\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe('> [!INFO]\n>\n>')
    })
  })

  describe('structural handling', () => {
    it('leaves content with no block directives untouched', () => {
      const input = 'Just a plain paragraph.\n\nAnother one.'

      expect(normalizeMarkdownBlocks(input)).toBe(input)
    })

    it('preserves indentation on both the block and its body', () => {
      const input = '   :::info\n   Body\n   :::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '   > [!INFO]\n   >\n   >    Body',
      )
    })

    it('normalizes CRLF line endings before parsing', () => {
      const input = ':::info\r\nBody\r\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe('> [!INFO]\n>\n> Body')
    })

    it('inserts a blank line before a block that follows a paragraph', () => {
      const input = 'Before\n:::info\nBody\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        'Before\n\n> [!INFO]\n>\n> Body',
      )
    })

    it('inserts a blank line after a block that precedes more content', () => {
      const input = ':::info\nBody\n:::\nAfter'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '> [!INFO]\n>\n> Body\n\nAfter',
      )
    })

    it('renders consecutive sibling blocks independently', () => {
      const input = ':::info\nFirst\n:::\n:::warning\nSecond\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '> [!INFO]\n>\n> First\n\n> [!WARNING]\n>\n> Second',
      )
    })

    it('protects a fenced code block nested inside a directive from being parsed as content', () => {
      const input = ':::info\n```\n:::\n```\nAfter fence\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(
        '> [!INFO]\n>\n> ```\n> :::\n> ```\n> After fence',
      )
    })

    it('does not parse ::: directives found inside a top-level fenced code block', () => {
      const input = '```\n:::info\nfake\n:::\n```'

      expect(normalizeMarkdownBlocks(input)).toBe(input)
    })

    it('leaves a nested (unbalanced) directive untouched as malformed', () => {
      const input = ':::info\nOuter start\n:::info\nNested\n:::\nOuter end\n:::'

      expect(normalizeMarkdownBlocks(input)).toBe(input)
    })

    it('leaves an unclosed block at end of file untouched, without duplicating its last line', () => {
      const input = ':::info\nUnclosed content'

      expect(normalizeMarkdownBlocks(input)).toBe(input)
    })
  })
})
