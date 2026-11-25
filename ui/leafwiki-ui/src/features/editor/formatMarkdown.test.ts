import { describe, expect, it } from 'vitest'
import { formatMarkdown } from './formatMarkdown'

describe('formatMarkdown', () => {
  it.each([
    ['empty doc stays empty', '', ''],
    ['blank-only doc becomes empty', '\n \n\t\n', ''],
    ['adds the final newline (MD047)', 'text', 'text\n'],
    ['drops extra trailing newlines (MD047)', 'text\n\n\n', 'text\n'],
    ['converts CRLF and CR to LF', 'a\r\nb\rc\r\n', 'a\nb\nc\n'],
    ['drops leading blank lines', '\n\ntext', 'text\n'],
    ['strips trailing spaces and tabs (MD009)', 'a \nb\t\nc \t ', 'a\nb\nc\n'],
    [
      'keeps a 2-space hard break (MD009)',
      'line one  \nline two',
      'line one  \nline two\n',
    ],
    [
      'shortens a longer hard break to 2 spaces (MD009)',
      'first   \nsecond',
      'first  \nsecond\n',
    ],
    ['strips whitespace on blank lines', 'a\n  \nb', 'a\n\nb\n'],
    ['collapses multiple blank lines (MD012)', 'a\n\n\n\nb', 'a\n\nb\n'],
    ['shrinks spaces after # (MD019)', '##   Title\t', '## Title\n'],
    [
      'adds a blank line around headings (MD022)',
      'text\n# H\ntext',
      'text\n\n# H\n\ntext\n',
    ],
    [
      'no blank line before a heading on the first line',
      '# H\ntext',
      '# H\n\ntext\n',
    ],
    [
      'no blank line after a heading on the last line',
      'text\n## H',
      'text\n\n## H\n',
    ],
    ['blank line between consecutive headings', '# A\n## B', '# A\n\n## B\n'],
    [
      'leaves #hashtags alone (no MD018)',
      'text\n#tag here',
      'text\n#tag here\n',
    ],
    ['leaves an indented # alone (no MD023)', 'text\n # H', 'text\n # H\n'],
    [
      'blank line between a heading and a fence',
      '# H\n```\ncode\n```',
      '# H\n\n```\ncode\n```\n',
    ],
  ])('%s', (_name, input, expected) => {
    expect(formatMarkdown(input)).toBe(expected)
  })

  describe('fenced code', () => {
    it.each([
      [
        'strips trailing whitespace, keeps blank lines',
        '```js  \nconst a = 1   \n\n\n\nconst b = 2  \n```  ',
        '```js\nconst a = 1\n\n\n\nconst b = 2\n```\n',
      ],
      [
        'leaves headings inside alone',
        '~~~\ntext\n#   not a heading\n~~~',
        '~~~\ntext\n#   not a heading\n~~~\n',
      ],
      [
        'a shorter or other-char fence does not close it',
        '````\n```\n~~~\n\n\n````\n\n\ntext',
        '````\n```\n~~~\n\n\n````\n\ntext\n',
      ],
      [
        'inside a list item',
        '- ~~~\n a  \n\n\n b\n ~~~\n# H\ntext',
        '- ~~~\n a\n\n\n b\n ~~~\n\n# H\n\ntext\n',
      ],
      [
        'inside a blockquote',
        '> ```\n> a\n>\n>\n> ```\n# H',
        '> ```\n> a\n>\n>\n> ```\n\n# H\n',
      ],
      [
        'a fence line with an info string does not close it',
        '```\n```js\n# x\n```',
        '```\n```js\n# x\n```\n',
      ],
    ])('%s', (_name, input, expected) => {
      expect(formatMarkdown(input)).toBe(expected)
    })
  })

  describe('indented code', () => {
    it.each([
      [
        'keeps blank lines and trailing spaces inside the block',
        'text\n\n    a = 1   \n\n\n    b = 2 \n\n\n\ntext',
        'text\n\n    a = 1   \n\n\n    b = 2 \n\ntext\n',
      ],
      [
        'a tab-indented block at the start of the doc',
        '\tcode \nmore',
        '\tcode \nmore\n',
      ],
      ['right after a heading', '# H\n    code ', '# H\n\n    code \n'],
      [
        'an indented line continuing a paragraph is text',
        'para\n    lazy ',
        'para\n    lazy\n',
      ],
      [
        'a list continuation is left alone',
        '- item\n\n    continued   \n- next',
        '- item\n\n    continued   \n- next\n',
      ],
    ])('%s', (_name, input, expected) => {
      expect(formatMarkdown(input)).toBe(expected)
    })
  })

  it('is idempotent', () => {
    const input =
      '\r\n# A  \r\ntext  \nmore \n\n\n##  B\n```\n  x  \n\n\n```\n\n    code  \n\n\n    more\n#tag'
    const once = formatMarkdown(input)
    expect(formatMarkdown(once)).toBe(once)
  })
})
