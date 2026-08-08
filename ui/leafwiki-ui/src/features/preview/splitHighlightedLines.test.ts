import { splitHighlightedLines } from './splitHighlightedLines'

describe('splitHighlightedLines', () => {
  it('splits plain text on newlines', () => {
    expect(splitHighlightedLines('a\nb\nc')).toEqual([['a'], ['b'], ['c']])
  })

  it('drops a trailing empty line from a final newline', () => {
    expect(splitHighlightedLines('a\nb\n')).toEqual([['a'], ['b']])
  })

  it('keeps a blank middle line', () => {
    expect(splitHighlightedLines('a\n\nb')).toEqual([['a'], [], ['b']])
  })
})
