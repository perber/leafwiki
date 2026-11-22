import { expect, test } from '@playwright/test';
import { createPageViaApi, loggedIn } from '../helpers/apiContext';
import LoginPage from '../pages/LoginPage';
import ViewPage from '../pages/ViewPage';
import { toAppPath } from '../pages/appPath';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

const paragraph =
  'A paragraph that fills the available width so the page is long enough to scroll. '.repeat(8);

interface ViewerLayout {
  scrollbarWidth: number;
  reservedWidth: number;
  offset: number;
}

// The subheader is a sibling of #scroll-container, so a scrollbar that takes up
// layout space narrows <main> but not the bar. In Safari that pushed the
// breadcrumbs half a scrollbar to the right of the article (#1549); the bar now
// reserves the width as its transparent right border. Keep the two boxes tied
// together.
test('breadcrumbs stay aligned with the article when a scrollbar takes up space', async ({
  page,
}) => {
  const api = await loggedIn();
  const slug = `scrollbar-alignment-${Date.now()}`;
  await createPageViaApi(api, {
    title: 'Scrollbar Alignment',
    slug,
    content: `# Scrollbar Alignment\n\n## First heading\n\n${paragraph}\n\n## Second heading\n\n${paragraph}\n`,
  });
  await api.dispose();

  // Short viewport: the seeded page is guaranteed to scroll, so the container
  // gets a vertical scrollbar.
  await page.setViewportSize({ width: 1600, height: 600 });
  await new LoginPage(page).loginViaApi(user, password);
  await page.goto(toAppPath(`/${slug}`));
  await new ViewPage(page).expectUserLoggedIn();
  await expect(page.locator('article>h1')).toHaveText('Scrollbar Alignment');

  // The TOC pane and the subheader's reserved padding animate their width
  // (index.css); measuring mid-transition compares two different frames.
  await page.waitForFunction(() =>
    document
      .getAnimations()
      .every(
        (animation) => !(animation instanceof CSSTransition) || animation.playState !== 'running',
      ),
  );

  const measure = () =>
    page.evaluate((): ViewerLayout => {
      const scroller = document.querySelector('#scroll-container') as HTMLElement | null;
      const subheader = document.querySelector('.page-viewer__subheader');
      const inner = document.querySelector('.page-viewer__subheader-inner');
      const content = document.querySelector('.page-viewer__content');
      if (!scroller || !subheader || !inner || !content) {
        throw new Error('the viewer is not rendered');
      }
      return {
        scrollbarWidth: scroller.offsetWidth - scroller.clientWidth,
        reservedWidth: parseFloat(getComputedStyle(subheader).borderRightWidth),
        offset: content.getBoundingClientRect().x - inner.getBoundingClientRect().x,
      };
    });

  const layout = await measure();

  if (layout.scrollbarWidth > 0) {
    // The real case: a scrollbar that takes up space must be reserved by the
    // subheader, or the two boxes drift apart by half its width.
    expect(layout.reservedWidth, 'the subheader reserves the scrollbar width').toBe(
      layout.scrollbarWidth,
    );
    expect(Math.abs(layout.offset), 'breadcrumbs and article start at the same x').toBeLessThan(1);
  } else {
    // Chromium and WebKit paint overlay scrollbars on macOS, which take no
    // space and never exercise the reservation. Drive the property the same way
    // AppLayout does, so the CSS half is still covered here.
    const before = await page.evaluate(
      () => document.querySelector('.page-viewer__subheader-inner')!.getBoundingClientRect().x,
    );
    await page.evaluate(() =>
      document.documentElement.style.setProperty('--content-scrollbar-width', '16px'),
    );
    const after = await page.evaluate(() => ({
      reservedWidth: parseFloat(
        getComputedStyle(document.querySelector('.page-viewer__subheader')!).borderRightWidth,
      ),
      x: document.querySelector('.page-viewer__subheader-inner')!.getBoundingClientRect().x,
    }));

    expect(after.reservedWidth, 'the subheader reserves the reserved width').toBe(16);
    expect(before - after.x, 'the inner row moves by half the reserved width').toBeCloseTo(8, 1);
  }
});
