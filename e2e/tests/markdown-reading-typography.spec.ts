import { expect, test } from '@playwright/test';

const surfaces = [
  { name: 'viewer', className: 'page-viewer__content', padded: false },
  {
    name: 'editor preview',
    className: 'markdown-editor__preview box-content h-full w-full min-w-full overflow-auto',
    padded: true,
  },
  { name: 'history preview', className: 'page-history__preview-body', padded: false },
];
const paragraph =
  'Readable prose keeps a comfortable line length and enough space between ideas. '.repeat(8);
const content = `<h1>Reading typography</h1><p id="first">${paragraph}</p><p id="second">${paragraph}</p>
<h2 id="heading">A related section</h2><p id="following">${paragraph}</p>
<ul id="list"><li>Readable lists share the prose measure.</li></ul>
<blockquote id="quote"><p>Quoted prose remains readable.</p></blockquote>
<div class="markdown-code-block" id="code"><pre><code>${'wide_code_sample_'.repeat(140)}</code></pre></div>
<div class="table-wrapper" id="table"><table><tbody><tr>${Array.from({ length: 12 }, (_, i) => `<td>Column ${i} content</td>`).join('')}</tr></tbody></table></div>
<p id="media"><a href="#image"><img alt="Wide illustration" width="1000" height="60" src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='1000' height='60'/%3E"></a></p>`;

// Exercise the renderer's DOM contract against the built production stylesheet.
for (const surface of surfaces) {
  for (const theme of ['light', 'dark']) {
    for (const width of [1200, 375]) {
      test(`${surface.name} ${theme} ${width}: readable prose with wide technical content`, async ({
        page,
      }) => {
        const response = await page.request.get('/');
        expect(response.ok()).toBe(true);
        const stylesheetLink = (await response.text()).match(
          /<link\b[^>]*\brel=["']stylesheet["'][^>]*>/,
        )?.[0];
        const href = stylesheetLink?.match(/\bhref=["']([^"']+)["']/)?.[1];
        expect(href, 'The served frontend includes its production stylesheet').toBeTruthy();
        const stylesheet = new URL(href!, response.url()).href;
        await page.setViewportSize({ width, height: 1000 });
        await page.route('**/__markdown-reading-typography', (route) =>
          route.fulfill({
            contentType: 'text/html',
            body: `<!doctype html><html class="${theme}"><head><link rel="stylesheet" href="${stylesheet}"></head>
<body><main style="margin:24px auto;padding:0 16px"><article class="${surface.className}">${surface.padded ? `<div class="p-4">${content}</div>` : content}</article></main></body></html>`,
          }),
        );
        await page.goto('/__markdown-reading-typography');
        const prose = page.locator('#first');
        const fontSize = width > 640 ? 17 : 16;
        await expect(prose).toHaveCSS('font-size', `${fontSize}px`);
        await expect(prose).toHaveCSS('line-height', `${Number((fontSize * 1.65).toFixed(2))}px`);
        const layout = await page.evaluate(() => {
          const rect = (id: string) => document.getElementById(id)!.getBoundingClientRect();
          const first = rect('first');
          const second = rect('second');
          const heading = rect('heading');
          return {
            proseWidths: ['first', 'second', 'heading', 'list', 'quote'].map(
              (id) => rect(id).width,
            ),
            paragraphGap: second.top - first.bottom,
            beforeHeading: heading.top - second.bottom,
            afterHeading: rect('following').top - heading.bottom,
            wideWidths: ['code', 'table', 'media'].map((id) => rect(id).width),
            imageWidth: document.querySelector('img')!.getBoundingClientRect().width,
            scrollableBlocks: ['#code pre', '#table'].map((selector) => {
              const element = document.querySelector(selector)!;
              return element.scrollWidth > element.clientWidth;
            }),
            overflow: document.documentElement.scrollWidth,
          };
        });
        for (const measure of layout.proseWidths) {
          expect(measure).toBeLessThanOrEqual(680);
          expect(measure).toBeGreaterThanOrEqual(width > 640 ? 679 : 300);
        }
        expect(layout.paragraphGap).toBeGreaterThanOrEqual(fontSize);
        expect(layout.paragraphGap).toBeLessThanOrEqual(20);
        expect(layout.beforeHeading).toBeGreaterThanOrEqual(32);
        expect(layout.afterHeading).toBeGreaterThanOrEqual(8);
        expect(layout.afterHeading).toBeLessThanOrEqual(12);
        expect(layout.afterHeading).toBeLessThan(layout.beforeHeading);
        if (width > 640) {
          for (const measure of layout.wideWidths) expect(measure).toBeGreaterThan(900);
          expect(layout.imageWidth).toBeGreaterThan(900);
        } else {
          expect(layout.scrollableBlocks).toEqual([true, true]);
        }
        expect(layout.overflow).toBeLessThanOrEqual(width);
        await page.emulateMedia({ media: 'print' });
        await expect(prose).toHaveCSS('font-size', '16px');
        await expect(prose).toHaveCSS('line-height', '26px');
        expect(
          await prose.evaluate((element) => element.getBoundingClientRect().width),
        ).toBeGreaterThan(width > 640 ? 900 : 300);
        await expect(page.locator('#heading')).toHaveCSS('break-after', 'avoid');
      });
    }
  }
}
