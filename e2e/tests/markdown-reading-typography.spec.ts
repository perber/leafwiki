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
        if (surface.name === 'viewer') {
          for (const measure of [...layout.wideWidths, layout.imageWidth]) {
            expect(measure).toBeLessThanOrEqual(680);
            expect(measure).toBeCloseTo(layout.proseWidths[0], 0);
          }
        } else if (width > 640) {
          for (const measure of layout.wideWidths) expect(measure).toBeGreaterThan(900);
          expect(layout.imageWidth).toBeGreaterThan(900);
        }
        if (surface.name === 'viewer' || width <= 640) {
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

for (const scenario of [
  { name: 'desktop', width: 1600, reserved: 0, className: '' },
  {
    name: 'desktop with TOC',
    width: 1600,
    reserved: 256,
    className: 'page-viewer__subheader--toc-reserved',
  },
  {
    name: 'desktop with collapsed TOC',
    width: 1600,
    reserved: 48,
    className: 'page-viewer__subheader--toc-reserved-collapsed',
  },
  { name: 'tablet', width: 768, reserved: 0, className: '' },
  { name: 'phone', width: 375, reserved: 0, className: '' },
]) {
  test(`viewer ${scenario.name}: chrome and nested content share the centered reading column`, async ({
    page,
  }) => {
    const response = await page.request.get('/');
    const href = (await response.text())
      .match(/<link\b[^>]*\brel=["']stylesheet["'][^>]*>/)?.[0]
      .match(/\bhref=["']([^"']+)["']/)?.[1];
    expect(href).toBeTruthy();
    const stylesheet = new URL(href!, response.url()).href;
    await page.setViewportSize({ width: scenario.width, height: 1000 });
    await page.route('**/__markdown-centered-viewer', (route) =>
      route.fulfill({
        contentType: 'text/html',
        body: `<!doctype html><html><head><link rel="stylesheet" href="${stylesheet}"></head><body>
      <div class="page-viewer__subheader ${scenario.className}"><div class="page-viewer__subheader-inner"><div class="page-viewer__subheader-main"><div class="page-viewer__subheader-copy"><nav class="breadcrumbs-nav"><ol class="breadcrumbs-nav__list"><li class="breadcrumbs-nav__item">Reading /</li><li class="breadcrumbs-nav__item"><span class="breadcrumbs-nav__current">${'A long current page title with several related details '.repeat(3)}</span></li></ol></nav><div class="page-viewer__metadata">Updated recently</div></div><button class="page-viewer__favorite-toggle">Favorite</button></div></div></div>
      <main style="width:calc(100% - ${scenario.reserved}px)">
      <div class="page-viewer__metadata-bar"><div class="page-viewer__metadata-bar-inner"><div class="page-metadata page-metadata--two-col"><div class="page-metadata__tags"><div class="page-metadata__tags-list"><button class="page-metadata__tag-chip">reading</button></div></div><div class="page-metadata__properties"><button class="page-metadata__props-toggle" aria-expanded="true">Properties</button><dl class="page-metadata__props-list"><div class="page-metadata__prop-row"><dt class="page-metadata__prop-key">description</dt><dd class="page-metadata__prop-value">${paragraph}</dd></div><div class="page-metadata__prop-row"><dt class="page-metadata__prop-key">url</dt><dd class="page-metadata__prop-value">https://example.com/${'LongUnbrokenValue'.repeat(12)}</dd></div></dl></div></div></div></div>
      <div class="page-viewer"><div class="page-viewer__body"><article class="page-viewer__content">${content}
      <div id="mixed"><p id="mixed-prose">Raw HTML prose</p><img alt="Mixed illustration" width="960" height="60" src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='960' height='60'/%3E"></div>
      <figure id="mixed-figure"><p id="figure-prose">Figure prose</p><img alt="Figure illustration" width="960" height="60" src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='960' height='60'/%3E"></figure>
      <ul id="nested-list"><li><p id="list-paragraph">${paragraph}</p><ul id="inner-list"><li>Nested item</li></ul></li></ul>
      <blockquote id="nested-quote"><p id="quote-paragraph">A short quotation.</p></blockquote>
      <aside class="markdown-shoutout markdown-shoutout--info" id="callout"><p class="markdown-shoutout__title">Note</p><div class="markdown-shoutout__content"><p id="callout-paragraph">A short note.</p></div></aside>
      </article><div class="backlinks__pane"><div class="backlinks__content"><div class="backlinks__group"><div class="backlinks__group-title">Referenced by <span class="backlinks__badge">1</span></div><ul><li class="backlinks__item"><a href="#">Related page</a></li></ul></div></div></div></div></div></main></body></html>`,
      }),
    );
    await page.goto('/__markdown-centered-viewer');
    const layout = await page.evaluate(() => {
      const box = (selector: string) => {
        const r = document.querySelector(selector)!.getBoundingClientRect();
        return { left: r.left, right: r.right, width: r.width, center: (r.left + r.right) / 2 };
      };
      const inset = (parent: string, child: string) => {
        const element = document.querySelector(parent)!;
        const style = getComputedStyle(element);
        return (
          box(child).left -
          box(parent).left -
          parseFloat(style.paddingLeft) -
          parseFloat(style.borderLeftWidth)
        );
      };
      return {
        main: box('main'),
        prose: [
          '#first',
          '#heading',
          '#list',
          '#quote',
          '#nested-list',
          '#nested-quote',
          '#callout',
        ].map(box),
        chrome: [
          '.page-viewer__subheader-main',
          '.page-viewer__metadata-bar-inner',
          '.page-viewer__body > .backlinks__pane',
        ].map(box),
        metadataValuesFit: Array.from(
          document.querySelectorAll('.page-metadata__prop-value'),
        ).every(
          (element) =>
            element.scrollWidth <= element.clientWidth &&
            element.getBoundingClientRect().left >=
              box('.page-viewer__metadata-bar-inner').left - 1 &&
            element.getBoundingClientRect().right <=
              box('.page-viewer__metadata-bar-inner').right + 1,
        ),
        headerFits:
          document.querySelector('.page-viewer__metadata')!.getBoundingClientRect().bottom <=
          document.querySelector('.page-viewer__subheader')!.getBoundingClientRect().bottom - 4,
        headerClearsMetadata:
          document.querySelector('.page-viewer__subheader')!.getBoundingClientRect().bottom <=
          document.querySelector('.page-viewer__metadata-bar-inner')!.getBoundingClientRect().top,
        wide: ['#code', '#table', '#media', '#mixed', '#mixed-figure'].map(box),
        mixedProse: ['#mixed-prose', '#figure-prose'].map(box),
        nestedInsets: [
          inset('#nested-list > li', '#list-paragraph'),
          inset('#nested-list > li', '#inner-list'),
          inset('#nested-quote', '#quote-paragraph'),
          inset('#callout', '#callout-paragraph'),
        ],
        overflow: document.documentElement.scrollWidth,
      };
    });
    expect
      .soft(layout.headerFits, 'Wrapped breadcrumbs and timestamp fit above the header border')
      .toBe(true);
    expect.soft(layout.headerClearsMetadata, 'Metadata starts below the header').toBe(true);
    expect
      .soft(layout.metadataValuesFit, 'Expanded property values wrap inside the metadata column')
      .toBe(true);
    for (const block of [...layout.prose, ...layout.wide]) {
      expect(Math.abs(block.center - layout.main.center)).toBeLessThan(1);
    }
    for (const block of layout.mixedProse) {
      expect(Math.abs(block.center - layout.main.center)).toBeLessThan(1);
      expect(block.width).toBeLessThanOrEqual(680);
    }
    for (const block of layout.prose) expect(block.width).toBeLessThanOrEqual(680);
    for (const block of layout.chrome) {
      expect(Math.abs(block.left - layout.prose[0].left)).toBeLessThan(1);
      // Chrome has no prose padding inside its aligned reading column.
      expect(block.width).toBeLessThanOrEqual(680);
    }
    for (const block of layout.wide) {
      expect(block.width).toBeLessThanOrEqual(680);
      expect(block.width).toBeCloseTo(layout.prose[0].width, 0);
    }
    for (const inset of layout.nestedInsets) expect(Math.abs(inset)).toBeLessThan(1);
    expect(layout.overflow).toBeLessThanOrEqual(scenario.width);
    await expect(page.locator('.breadcrumbs-nav')).toHaveCSS('font-size', '12px');
    await expect(page.locator('.page-viewer__metadata')).toHaveCSS('font-size', '11px');
    if (scenario.width >= 768) {
      const shortHeader = await page.evaluate(() => {
        document.querySelector('.breadcrumbs-nav__current')!.textContent = 'Current page';
        const header = document.querySelector('.page-viewer__subheader')!.getBoundingClientRect();
        const copy = document
          .querySelector('.page-viewer__subheader-copy')!
          .getBoundingClientRect();
        return {
          height: header.height,
          centerOffset: (copy.top + copy.bottom - header.top - header.bottom) / 2,
        };
      });
      expect(shortHeader.height).toBeGreaterThanOrEqual(48);
      expect(Math.abs(shortHeader.centerOffset)).toBeLessThan(1);
    }
    await page.emulateMedia({ media: 'print' });
    const printed = await page.locator('#first').boundingBox();
    const article = await page.locator('article').boundingBox();
    expect(Math.abs(printed!.x - article!.x)).toBeLessThan(1);
    expect(Math.abs(printed!.width - article!.width)).toBeLessThan(1);
    if (scenario.width >= 1600) expect(article!.width).toBeGreaterThan(680);
  });
}
