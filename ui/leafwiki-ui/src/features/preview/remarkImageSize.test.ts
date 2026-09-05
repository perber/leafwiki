import type { Paragraph, Root } from 'mdast'

import { describe, expect, it } from 'vitest'

import { remarkImageSize } from './remarkImageSize'

function transform(children: Root['children']): Root {
  const tree: Root = {
    type: 'root',
    children,
  }

  remarkImageSize()(tree)

  return tree
}

function getParagraph(tree: Root): Paragraph {
  const node = tree.children[0]

  if (node.type !== 'paragraph') {
    throw new Error(`Expected paragraph, got ${node.type}`)
  }

  return node
}

function imageWithWidth(value: string, imageData?: object) {
  return transform([
    {
      type: 'paragraph',
      children: [
        {
          type: 'image',
          url: '/image.png',
          alt: 'image',
          ...(imageData ? { data: imageData } : {}),
        },
        {
          type: 'text',
          value,
        },
      ],
    },
  ])
}

describe('remarkImageSize', () => {
  it.each(['1%', '50%', '75%', '100%', '12.5%'])(
    'sets the image width to %s',
    (width) => {
      const tree = imageWithWidth(`{width=${width}}`)
      const paragraph = getParagraph(tree)

      expect(paragraph).toMatchObject({
        type: 'paragraph',
        children: [
          {
            type: 'image',
            data: {
              hProperties: {
                width,
              },
            },
          },
        ],
      })
    },
  )

  it('removes the width marker when it is the only following text', () => {
    const tree = imageWithWidth('{width=75%}')
    const paragraph = getParagraph(tree)

    expect(paragraph.children).toHaveLength(1)
    expect(paragraph.children[0]).toMatchObject({
      type: 'image',
    })
  })

  it('preserves text following the width marker', () => {
    const tree = imageWithWidth('{width=75%} caption')
    const paragraph = getParagraph(tree)

    expect(paragraph.children).toHaveLength(2)
    expect(paragraph.children[1]).toMatchObject({
      type: 'text',
      value: ' caption',
    })
  })

  it('allows whitespace before the width marker', () => {
    const tree = imageWithWidth('   {width=75%}')

    expect(tree.children[0]).toMatchObject({
      children: [
        {
          type: 'image',
          data: {
            hProperties: {
              width: '75%',
            },
          },
        },
      ],
    })
  })

  it.each(['{width=0%}', '{width=101%}', '{width=200%}'])(
    'consumes out-of-range width %s without resizing the image',
    (width) => {
      const tree = imageWithWidth(width)
      const paragraph = getParagraph(tree)
      const image = paragraph.children[0]

      expect(image).toMatchObject({
        type: 'image',
      })
      expect(image).not.toHaveProperty('data')
      expect(paragraph.children).toHaveLength(1)
    },
  )

  it.each(['{width=-1%}', '{width=abc%}', '{width=75}', '{width=75%%}'])(
    'consumes invalid width syntax %s without resizing the image',
    (width) => {
      const tree = imageWithWidth(width)
      const paragraph = getParagraph(tree)
      const image = paragraph.children[0]

      expect(image).toMatchObject({
        type: 'image',
      })
      expect(image).not.toHaveProperty('data')
      expect(paragraph.children).toHaveLength(1)
    },
  )

  it.each([
    '{width=0%} caption',
    '{width=101%} caption',
    '{width=200%} caption',
    '{width=-1%} caption',
    '{width=abc%} caption',
    '{width=75} caption',
    '{width=75%%} caption',
  ])(
    'consumes an invalid width marker and preserves trailing text: %s',
    (value) => {
      const tree = imageWithWidth(value)
      const paragraph = getParagraph(tree)

      expect(paragraph.children).toHaveLength(2)
      expect(paragraph.children[0]).toMatchObject({
        type: 'image',
      })
      expect(paragraph.children[0]).not.toHaveProperty('data')
      expect(paragraph.children[1]).toMatchObject({
        type: 'text',
        value: ' caption',
      })
    },
  )

  it('does not resize an image when the next node is not text', () => {
    const tree = transform([
      {
        type: 'paragraph',
        children: [
          {
            type: 'image',
            url: '/image.png',
            alt: 'image',
          },
          {
            type: 'strong',
            children: [{ type: 'text', value: '{width=75%}' }],
          },
        ],
      },
    ])

    expect(tree.children[0]).toMatchObject({
      children: [
        {
          type: 'image',
        },
        {
          type: 'strong',
        },
      ],
    })
  })

  it('preserves existing image properties', () => {
    const tree = imageWithWidth('{width=75%}', {
      hProperties: {
        className: ['custom-image'],
      },
    })
    const paragraph = getParagraph(tree)
    const image = paragraph.children[0]

    expect(image).toMatchObject({
      data: {
        hProperties: {
          className: ['custom-image'],
          width: '75%',
        },
      },
    })
  })
})
