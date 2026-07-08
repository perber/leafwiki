import { describe, expect, it } from 'vitest'
import { htmlToMarkdown } from './htmlToMarkdown'

describe('htmlToMarkdown', () => {
  describe('headings', () => {
    it('converts h1', () => {
      expect(htmlToMarkdown('<h1>Title</h1>')).toBe('# Title')
    })

    it('converts h2', () => {
      expect(htmlToMarkdown('<h2>Section</h2>')).toBe('## Section')
    })

    it('converts h3', () => {
      expect(htmlToMarkdown('<h3>Subsection</h3>')).toBe('### Subsection')
    })

    it('converts h4', () => {
      expect(htmlToMarkdown('<h4>Deep</h4>')).toBe('#### Deep')
    })

    it('converts h5', () => {
      expect(htmlToMarkdown('<h5>Deeper</h5>')).toBe('##### Deeper')
    })

    it('converts h6', () => {
      expect(htmlToMarkdown('<h6>Deepest</h6>')).toBe('###### Deepest')
    })
  })

  describe('inline formatting', () => {
    it('converts bold', () => {
      expect(htmlToMarkdown('<p><strong>bold</strong></p>')).toBe('**bold**')
    })

    it('converts italic', () => {
      expect(htmlToMarkdown('<p><em>italic</em></p>')).toBe('_italic_')
    })

    it('converts b tag as bold', () => {
      expect(htmlToMarkdown('<p><b>bold</b></p>')).toBe('**bold**')
    })

    it('converts i tag as italic', () => {
      expect(htmlToMarkdown('<p><i>italic</i></p>')).toBe('_italic_')
    })

    it('converts strikethrough', () => {
      expect(htmlToMarkdown('<p><s>strikethrough</s></p>')).toBe(
        '~~strikethrough~~',
      )
    })

    it('converts del tag as strikethrough', () => {
      expect(htmlToMarkdown('<p><del>deleted</del></p>')).toBe('~~deleted~~')
    })

    it('converts inline code', () => {
      expect(htmlToMarkdown('<p><code>const x = 1</code></p>')).toBe(
        '`const x = 1`',
      )
    })
  })

  describe('links', () => {
    it('converts anchor tags to inline links', () => {
      expect(htmlToMarkdown('<a href="https://example.com">Example</a>')).toBe(
        '[Example](https://example.com)',
      )
    })

    it('converts link with title attribute', () => {
      expect(
        htmlToMarkdown(
          '<a href="https://example.com" title="My Site">Example</a>',
        ),
      ).toBe('[Example](https://example.com "My Site")')
    })

    it('ignores anchors without href', () => {
      expect(htmlToMarkdown('<a name="anchor">Text</a>')).toBe('Text')
    })
  })

  describe('images', () => {
    it('converts img tags', () => {
      expect(
        htmlToMarkdown(
          '<img src="https://example.com/img.png" alt="A picture">',
        ),
      ).toBe('![A picture](https://example.com/img.png)')
    })

    it('converts img without alt as empty alt', () => {
      expect(htmlToMarkdown('<img src="https://example.com/img.png">')).toBe(
        '![](https://example.com/img.png)',
      )
    })
  })

  describe('unordered lists', () => {
    it('converts simple unordered list', () => {
      const html = '<ul><li>Alpha</li><li>Beta</li><li>Gamma</li></ul>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('- Alpha\n- Beta\n- Gamma')
    })

    it('converts nested unordered list', () => {
      const html = '<ul><li>Parent<ul><li>Child</li></ul></li></ul>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('- Parent\n    - Child')
    })
  })

  describe('ordered lists', () => {
    it('converts simple ordered list', () => {
      const html = '<ol><li>First</li><li>Second</li><li>Third</li></ol>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('1. First\n2. Second\n3. Third')
    })

    it('converts nested ordered list', () => {
      const html = '<ol><li>First<ol><li>Sub-first</li></ol></li></ol>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('1. First\n    1. Sub-first')
    })
  })

  describe('task lists', () => {
    it('converts unchecked checkbox list item', () => {
      const html = '<ul><li><input type="checkbox"> Todo item</li></ul>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('- [ ] Todo item')
    })

    it('converts checked checkbox list item', () => {
      const html = '<ul><li><input type="checkbox" checked> Done item</li></ul>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('- [x] Done item')
    })
  })

  describe('code blocks', () => {
    it('converts pre/code blocks with fenced syntax', () => {
      const html =
        '<pre><code>function hello() {\n  return "world"\n}</code></pre>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('```\nfunction hello() {\n  return "world"\n}\n```')
    })
  })

  describe('blockquotes', () => {
    it('converts blockquote', () => {
      const html = '<blockquote><p>Quoted text</p></blockquote>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('> Quoted text')
    })

    it('converts nested blockquote', () => {
      const html =
        '<blockquote><p>Outer</p><blockquote><p>Inner</p></blockquote></blockquote>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('> Outer\n> \n> > Inner')
    })
  })

  describe('tables', () => {
    it('converts simple table', () => {
      const html = `
        <table>
          <thead><tr><th>Name</th><th>Age</th></tr></thead>
          <tbody><tr><td>Alice</td><td>30</td></tr></tbody>
        </table>
      `
      const result = htmlToMarkdown(html)
      expect(result).toBe('| Name | Age |\n| --- | --- |\n| Alice | 30 |')
    })

    it('converts table with multiple rows', () => {
      const html = `
        <table>
          <thead><tr><th>Item</th><th>Value</th></tr></thead>
          <tbody>
            <tr><td>A</td><td>1</td></tr>
            <tr><td>B</td><td>2</td></tr>
          </tbody>
        </table>
      `
      const result = htmlToMarkdown(html)
      expect(result).toBe(
        '| Item | Value |\n| --- | --- |\n| A | 1 |\n| B | 2 |',
      )
    })

    // Many real paste sources (Google Sheets, plain web tables, Excel-to-HTML)
    // render every cell as <td> with no <th> at all. turndown-plugin-gfm only
    // recognizes a header row made of literal <th> cells and otherwise keeps
    // the whole table as raw, unconverted HTML.
    it('converts table with no th header cells', () => {
      const html = `
        <table>
          <tbody>
            <tr><td>Name</td><td>Age</td></tr>
            <tr><td>Alice</td><td>30</td></tr>
          </tbody>
        </table>
      `
      const result = htmlToMarkdown(html)
      expect(result).toBe('| Name | Age |\n| --- | --- |\n| Alice | 30 |')
    })

    it('converts table with no thead/tbody at all', () => {
      const html = `
        <table>
          <tr><td>Name</td><td>Age</td></tr>
          <tr><td>Alice</td><td>30</td></tr>
        </table>
      `
      const result = htmlToMarkdown(html)
      expect(result).toBe('| Name | Age |\n| --- | --- |\n| Alice | 30 |')
    })
  })

  describe('paragraphs', () => {
    it('converts paragraph text', () => {
      expect(htmlToMarkdown('<p>Hello world</p>')).toBe('Hello world')
    })

    it('converts multiple paragraphs with blank line separator', () => {
      const html = '<p>First paragraph</p><p>Second paragraph</p>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('First paragraph\n\nSecond paragraph')
    })
  })

  describe('horizontal rule', () => {
    it('converts hr to ---', () => {
      expect(htmlToMarkdown('<hr>')).toBe('---')
    })
  })

  describe('Google Docs clipboard', () => {
    it('strips the Google Docs <b font-weight:normal> wrapper', () => {
      const html =
        '<b style="font-weight:normal;" id="docs-internal-guid-abc123"><p>Hello world</p></b>'
      expect(htmlToMarkdown(html)).toBe('Hello world')
    })

    it('preserves actual bold inside Google Docs wrapper', () => {
      const html =
        '<b style="font-weight:normal;" id="docs-internal-guid-abc123"><p>Hello <b>bold</b> world</p></b>'
      expect(htmlToMarkdown(html)).toBe('Hello **bold** world')
    })

    it('treats font-weight:400 the same as normal', () => {
      const html = '<b style="font-weight:400;">Just a wrapper</b>'
      expect(htmlToMarkdown(html)).toBe('Just a wrapper')
    })

    it('unwraps Google Docs redirect links', () => {
      const html =
        '<a href="https://www.google.com/url?q=https%3A%2F%2Fexample.com&sa=D&source=docs">Example</a>'
      expect(htmlToMarkdown(html)).toBe('[Example](https://example.com)')
    })

    it('leaves non-Google links unchanged', () => {
      const html = '<a href="https://example.com">Example</a>'
      expect(htmlToMarkdown(html)).toBe('[Example](https://example.com)')
    })

    it('strips list prefix from headings wrapped in ol/li with list-style-type:none', () => {
      const html = `<b style="font-weight:normal;" id="docs-internal-guid-abc">
        <ol><li dir="ltr" style="list-style-type:none;font-size:20pt;font-weight:700;">
          <p dir="ltr"><span style="font-size:20pt;font-weight:700;">My Heading</span></p>
        </li></ol>
      </b>`
      // No "1. " prefix — list-style-type:none marks this as a structural wrapper, not a real list
      expect(htmlToMarkdown(html)).toBe('My Heading')
    })

    it('converts h1 inside ol/li with list-style-type:none without list prefix', () => {
      const html = `<ol><li style="list-style-type:none;"><h1>Section Title</h1></li></ol>`
      expect(htmlToMarkdown(html)).toBe('# Section Title')
    })
  })

  describe('Microsoft Word / Outlook clipboard', () => {
    it('removes <o:p> tags', () => {
      expect(htmlToMarkdown('<p>Hello<o:p></o:p></p>')).toBe('Hello')
    })

    it('removes <o:p> with attributes', () => {
      expect(htmlToMarkdown('<p>Hello<o:p class="x"></o:p></p>')).toBe('Hello')
    })
  })

  describe('non-breaking spaces', () => {
    it('converts &nbsp; to regular space', () => {
      expect(htmlToMarkdown('<p>Hello&nbsp;world</p>')).toBe('Hello world')
    })

    it('converts multiple &nbsp; to regular spaces', () => {
      expect(htmlToMarkdown('<p>a&nbsp;&nbsp;b</p>')).toBe('a  b')
    })

    it('preserves &nbsp; inside fenced code blocks instead of converting it', () => {
      const html = '<pre><code>a&nbsp;&nbsp;b</code></pre>'
      expect(htmlToMarkdown(html)).toBe('```\na  b\n```')
    })

    it('preserves &nbsp; inside inline code instead of converting it', () => {
      const html = '<p><code>a&nbsp;b</code></p>'
      expect(htmlToMarkdown(html)).toBe('`a b`')
    })
  })

  describe('mixed content (Word/OneNote-like)', () => {
    it('converts a document with heading, paragraph, and list', () => {
      const html = `
        <h1>Meeting Notes</h1>
        <p>Summary of today's meeting.</p>
        <ul>
          <li>Action item 1</li>
          <li>Action item 2</li>
        </ul>
      `
      const result = htmlToMarkdown(html)
      expect(result).toBe(
        "# Meeting Notes\n\nSummary of today's meeting.\n\n- Action item 1\n- Action item 2",
      )
    })

    it('converts a document with heading, bold text, and ordered list', () => {
      const html = `
        <h2>Steps</h2>
        <p>Follow these <strong>important</strong> steps:</p>
        <ol>
          <li>Open the app</li>
          <li>Click <em>Settings</em></li>
        </ol>
      `
      const result = htmlToMarkdown(html)
      expect(result).toBe(
        '## Steps\n\nFollow these **important** steps:\n\n1. Open the app\n2. Click _Settings_',
      )
    })

    it('strips style and script elements', () => {
      const html =
        '<style>body { color: red; }</style><p>Text</p><script>alert(1)</script>'
      const result = htmlToMarkdown(html)
      expect(result).toBe('Text')
    })

    it('handles empty input', () => {
      expect(htmlToMarkdown('')).toBe('')
    })

    it('handles plain text without HTML tags', () => {
      expect(htmlToMarkdown('plain text')).toBe('plain text')
    })
  })
})
