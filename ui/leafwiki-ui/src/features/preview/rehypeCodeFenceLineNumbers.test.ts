import { rehypeCodeFenceLineNumbers } from './rehypeCodeFenceLineNumbers'
import type { Root } from 'hast'

function run(tree: Root) {
  const attacher = rehypeCodeFenceLineNumbers as unknown as () => (
    tree: Root,
  ) => void | Root | Promise<void | Root>
  const transformer = attacher()
  return transformer(tree)
}

describe('rehypeCodeFenceLineNumbers', () => {
  it('strips trailing = from language class and sets data-line-numbers', () => {
    const tree: Root = {
      type: 'root',
      children: [
        {
          type: 'element',
          tagName: 'code',
          properties: { className: ['language-js='] },
          children: [],
        },
      ],
    }

    run(tree)

    const code = tree.children[0] as unknown as {
      properties: { className: string[]; 'data-line-numbers'?: string }
    }
    expect(code.properties.className).toEqual(['language-js'])
    expect(code.properties['data-line-numbers']).toBe('true')
  })

  it('leaves ordinary language classes alone', () => {
    const tree: Root = {
      type: 'root',
      children: [
        {
          type: 'element',
          tagName: 'code',
          properties: { className: ['language-js'] },
          children: [],
        },
      ],
    }

    run(tree)

    const code = tree.children[0] as unknown as {
      properties: { className: string[]; 'data-line-numbers'?: string }
    }
    expect(code.properties.className).toEqual(['language-js'])
    expect(code.properties['data-line-numbers']).toBeUndefined()
  })
})
