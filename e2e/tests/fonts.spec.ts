import { expect, test } from '@playwright/test';

const faces = [
  { name: 'body', weight: '400', style: 'normal' },
  { name: 'medium UI labels', weight: '500', style: 'normal' },
  { name: 'semibold UI labels', weight: '600', style: 'normal' },
  { name: 'bold text', weight: '700', style: 'normal' },
  { name: 'prose headings', weight: '800', style: 'normal' },
  { name: 'strong prose headings', weight: '900', style: 'normal' },
  { name: 'emphasis', weight: '400', style: 'italic' },
  { name: 'blockquotes', weight: '500', style: 'italic' },
  { name: 'strong emphasis', weight: '600', style: 'italic' },
  { name: 'bold emphasis', weight: '700', style: 'italic' },
];

test.describe('Self-hosted typography', () => {
  test.beforeEach(async ({ page }) => {
    // Typography must work without an authenticated session or a reachable API.
    await page.route(
      (url) => url.pathname.startsWith('/api/'),
      (route) => route.abort(),
    );
    await page.goto('/');
    await expect(page.locator('body')).toHaveCSS('font-family', /Inter/);
  });

  for (const { name, weight, style } of faces) {
    test(`renders ${name} using a bundled Inter face`, async ({ page }) => {
      const loaded = await page.evaluate(
        async ({ weight, style }) => {
          const family = getComputedStyle(document.body).fontFamily.split(',')[0].trim();
          const matching = await document.fonts.load(
            `${style} ${weight} 16px ${family}`,
            'LeafWiki',
          );
          return matching.map((face) => ({
            family: face.family.replaceAll('"', '').replaceAll("'", ''),
            weight: face.weight,
            style: face.style,
            status: face.status,
          }));
        },
        { weight, style },
      );

      expect(loaded).toContainEqual({ family: 'Inter', weight, style, status: 'loaded' });
    });
  }

  test('draws inherited text at each used weight with Inter instead of a system fallback', async ({
    page,
    browserName,
  }) => {
    test.skip(browserName !== 'chromium', 'Rendered font inspection requires Chromium CDP');
    await page.evaluate(async (faces) => {
      for (const [index, { weight, style }] of faces.entries()) {
        const paragraph = document.createElement('p');
        paragraph.id = `font-regression-text-${index}`;
        paragraph.textContent = 'LeafWiki typography';
        paragraph.style.fontWeight = weight;
        paragraph.style.fontStyle = style;
        document.body.append(paragraph);
      }
      await document.fonts.ready;
    }, faces);

    const session = await page.context().newCDPSession(page);
    await session.send('DOM.enable');
    await session.send('CSS.enable');
    const { root } = await session.send('DOM.getDocument');
    for (const [index, { name }] of faces.entries()) {
      const { nodeId } = await session.send('DOM.querySelector', {
        nodeId: root.nodeId,
        selector: `#font-regression-text-${index}`,
      });
      const { fonts } = await session.send('CSS.getPlatformFontsForNode', { nodeId });
      expect(fonts, name).toEqual([
        expect.objectContaining({
          familyName: expect.stringMatching(/^Inter(?: |$)/),
          isCustomFont: true,
        }),
      ]);
    }
    await session.detach();
  });
});
