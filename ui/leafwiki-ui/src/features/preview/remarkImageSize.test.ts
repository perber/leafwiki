import type { Root } from 'mdast'

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

function imageWithSize(value: string, imageData?: object) {
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
    (size) => {
      const tree = imageWithSize(`{size=${size}}`)
      const paragraph = tree.children[0]

      expect(paragraph).toMatchObject({
        type: 'paragraph',
        children: [
          {
            type: 'image',
            data: {
              hProperties: {
                width: size,
              },
            },
          },
        ],
      })
    },
  )

  it('removes the size marker when it is the only following text', () => {
    const tree = imageWithSize('{size=75%}')
    const paragraph = tree.children[0]

    expect(paragraph.children).toHaveLength(1)
    expect(paragraph.children[0]).toMatchObject({
      type: 'image',
    })
  })

  it('preserves text following the size marker', () => {
    const tree = imageWithSize('{size=75%} caption')
    const paragraph = tree.children[0]

    expect(paragraph.children).toHaveLength(2)
    expect(paragraph.children[1]).toMatchObject({
      type: 'text',
      value: ' caption',
    })
  })

  it('allows whitespace before the size marker', () => {
    const tree = imageWithSize('   {size=75%}')

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

  it.each(['{size=0%}', '{size=101%}', '{size=200%}'])(
    'ignores out-of-range size %s',
    (size) => {
      const tree = imageWithSize(size)
      const paragraph = tree.children[0]
      const image = paragraph.children[0]

      expect(image).toMatchObject({
        type: 'image',
      })
      expect((image as { data?: { hProperties?: object } }).data).toBeUndefined()
      expect(paragraph.children[1]).toMatchObject({
        type: 'text',
        value: size,
      })
    },
  )

  it.each([
    '{size=-1%}',
    '{size=abc%}',
    '{size=75}',
    '{size=75%%}',
  ])('ignores invalid size syntax %s', (size) => {
    const tree = imageWithSize(size)
    const paragraph = tree.children[0]
    const image = paragraph.children[0]

    expect(image).toMatchObject({
      type: 'image',
    })
    expect((image as { data?: { hProperties?: object } }).data).toBeUndefined()
    expect(paragraph.children[1]).toMatchObject({
      type: 'text',
      value: size,
    })
  })

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
            children: [{ type: 'text', value: '{size=75%}' }],
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
    const tree = imageWithSize('{size=75%}', {
      hProperties: {
        className: ['custom-image'],
      },
    })
    const image = tree.children[0].children[0]

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