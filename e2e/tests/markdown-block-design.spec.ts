import { expect, test } from '@playwright/test';

const surfaces = ['page-viewer__content', 'markdown-editor__preview', 'page-history__preview-body'];
const variants = [
  'info',
  'success',
  'warning',
  'error',
  'blue',
  'red',
  'green',
  'orange',
  'custom',
];
const content = `<p>Keep <strong>important text</strong> and <em>emphasis</em> readable.</p>
<ul><li>First item</li><li>Second item</li></ul><pre><code>const example = 'A long code sample stays within the panel and scrolls when needed';</code></pre>`;

// Use the production stylesheet with the renderer's existing DOM contract, without a database.
for (const surface of surfaces) {
  for (const theme of ['light', 'dark']) {
    for (const width of [1100, 375]) {
      test(`${surface} ${theme} ${width}: understated panels and native disclosure controls`, async ({
        page,
      }) => {
        const indexResponse = await page.request.get('/');
        expect(indexResponse.ok()).toBe(true);
        const indexHtml = await indexResponse.text();
        const stylesheetLink = indexHtml.match(/<link\b[^>]*\brel=["']stylesheet["'][^>]*>/)?.[0];
        const stylesheetHref = stylesheetLink?.match(/\bhref=["']([^"']+)["']/)?.[1];
        expect(
          stylesheetHref,
          'The served frontend includes its production stylesheet',
        ).toBeTruthy();
        const stylesheet = new URL(stylesheetHref!, indexResponse.url()).href;
        await page.setViewportSize({ width, height: 1100 });
        await page.emulateMedia({ reducedMotion: 'reduce' });
        await page.route('**/__markdown-block-design', (route) =>
          route.fulfill({
            contentType: 'text/html',
            body: `<!doctype html><html class="${theme}"><head><link rel="stylesheet" href="${stylesheet}"></head>
<body><main style="max-width:760px;margin:24px auto;padding:0 16px"><article class="${surface} prose">
${variants.map((variant, index) => `<aside class="markdown-shoutout markdown-shoutout--${variant}">${index < 4 || index % 2 === 0 ? `<p class="markdown-shoutout__title">${variant} notice</p>` : ''}<div class="markdown-shoutout__content"><p>Original body formatting stays normal.</p></div></aside>`).join('')}
<aside class="markdown-shoutout markdown-shoutout--info"><div class="markdown-shoutout__content"><p>An untitled semantic note</p></div></aside>
<details class="markdown-collapsible" id="disclosure"><summary>A readable long summary ${'with context '.repeat(8)}</summary>${content}
<aside class="markdown-shoutout markdown-shoutout--info"><div class="markdown-shoutout__content"><p>Nested note</p></div></aside>
<details class="markdown-collapsible"><summary>Nested disclosure</summary><p>Nested content</p></details></details>
<details class="markdown-collapsible"><p>Untitled native disclosure</p></details>
<details id="raw"><summary>Raw HTML details</summary><p>Unchanged native rendering</p></details>
</article></main></body></html>`,
          }),
        );
        await page.goto('/__markdown-block-design');
        const panels = page.locator('article > .markdown-shoutout');
        for (const panel of await panels.all()) {
          await expect(panel).toHaveCSS('border-top-width', '0px');
          await expect(panel).toHaveCSS('border-right-width', '0px');
          await expect(panel).toHaveCSS('border-bottom-width', '0px');
          await expect(panel).toHaveCSS('border-left-width', '3px');
          await expect(panel).toHaveCSS('padding-top', '16px');
          await expect(panel).toHaveCSS('padding-left', '20px');
          await expect(panel.locator('.markdown-shoutout__content')).toHaveCSS(
            'font-style',
            'normal',
          );
        }
        const colors = await panels.evaluateAll((elements) =>
          elements.slice(0, 8).map((element) => getComputedStyle(element).borderLeftColor),
        );
        expect(new Set(colors).size).toBe(8);
        await expect(page.locator('.markdown-shoutout__title').first()).toHaveCSS(
          'text-transform',
          'none',
        );
        const titleContrasts = await page
          .locator('article > aside > .markdown-shoutout__title')
          .evaluateAll((titles) => {
            const canvas = document.createElement('canvas');
            canvas.width = canvas.height = 1;
            const context = canvas.getContext('2d')!;
            const luminance = (pixels: Uint8ClampedArray) => {
              const channels = [...pixels.slice(0, 3)].map((channel) => {
                const value = channel / 255;
                return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
              });
              return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
            };
            return titles.map((title) => {
              const ancestors: Element[] = [];
              for (let node: Element | null = title; node; node = node.parentElement)
                ancestors.unshift(node);
              context.fillStyle = '#fff';
              context.fillRect(0, 0, 1, 1);
              for (const ancestor of ancestors) {
                context.fillStyle = getComputedStyle(ancestor).backgroundColor;
                context.fillRect(0, 0, 1, 1);
              }
              const background = luminance(context.getImageData(0, 0, 1, 1).data);
              context.fillStyle = getComputedStyle(title).color;
              context.fillRect(0, 0, 1, 1);
              const foreground = luminance(context.getImageData(0, 0, 1, 1).data);
              return {
                title: title.textContent,
                ratio:
                  (Math.max(background, foreground) + 0.05) /
                  (Math.min(background, foreground) + 0.05),
              };
            });
          });
        for (const { title, ratio } of titleContrasts)
          expect(ratio, `${title} contrast`).toBeGreaterThanOrEqual(4.5);
        const disclosure = page.locator('#disclosure');
        const summary = disclosure.locator(':scope > summary');
        await expect(disclosure).toHaveCSS('border-top-width', '0px');
        await expect(summary).toHaveCSS('margin-top', '0px');
        await expect(summary).toHaveCSS('margin-left', '0px');
        await expect(summary).toHaveCSS('min-height', '28px');
        await summary.focus();
        await expect(summary).toHaveCSS('outline-width', '2px');
        const idleBackground = await summary.evaluate(
          (element) => getComputedStyle(element).backgroundColor,
        );
        await summary.hover();
        expect(
          await summary.evaluate((element) => getComputedStyle(element).backgroundColor),
        ).not.toBe(idleBackground);
        await summary.press('Enter');
        await expect(disclosure).toHaveAttribute('open', '');
        const spacing = await disclosure.evaluate((element) => {
          const summary = element.querySelector('summary')!.getBoundingClientRect();
          const paragraph = element.querySelector('p')!.getBoundingClientRect();
          return paragraph.top - summary.bottom;
        });
        expect(spacing).toBeGreaterThanOrEqual(16);
        await expect(disclosure.locator('strong')).toHaveCSS('font-weight', '600');
        await expect(disclosure.locator('em')).toHaveCSS('font-style', 'italic');
        expect(
          await summary.evaluate(
            (element) => getComputedStyle(element, '::before').transitionProperty,
          ),
        ).toBe('none');
        await summary.press('Space');
        await expect(disclosure).not.toHaveAttribute('open');
        await summary.click();
        await expect(disclosure).toHaveAttribute('open', '');
        const raw = page.locator('#raw');
        await expect(raw).toHaveCSS('border-left-width', '0px');
        await page
          .locator('article > details.markdown-collapsible')
          .nth(1)
          .evaluate((element) => element.setAttribute('open', ''));
        expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
          width,
        );
        if (surface === 'page-viewer__content')
          await page.screenshot({
            path: test.info().outputPath(`blocks-${theme}-${width}.png`),
            fullPage: true,
          });
      });
    }
  }
}
