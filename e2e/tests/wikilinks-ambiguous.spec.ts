import { expect, test } from '@playwright/test';
import { createPageViaApi, loggedIn } from '../helpers/apiContext';

// #1615: a [[Title]] link matching two pages is ambiguous, not broken. It must
// show up as a (non-broken) outgoing link and stay off the broken-links list.
test('ambiguous wikilink is not reported as broken', async () => {
  const api = await loggedIn();
  const stamp = Date.now();
  const title = `Twin ${stamp}`;
  const missing = `Missing ${stamp}`;

  await createPageViaApi(api, { title, slug: `twin-a-${stamp}`, content: 'a' });
  await createPageViaApi(api, { title, slug: `twin-b-${stamp}`, content: 'b' });
  const srcId = await createPageViaApi(api, {
    title: `Source ${stamp}`,
    slug: `source-${stamp}`,
    content: `See [[${title}]] and [[${missing}]].`,
  });

  const links = await (await api.get(`/api/pages/${srcId}/links`)).json();
  const outgoing = (links.outgoings ?? []).find((l: { to_path: string }) => l.to_path === title);
  expect(outgoing, 'ambiguous link listed as outgoing').toBeDefined();
  expect(outgoing.broken).toBe(false);
  const brokenOut = (links.broken_outgoings ?? []).map((l: { to_path: string }) => l.to_path);
  expect(brokenOut).toEqual([missing]);

  const broken = await (await api.get('/api/links/broken')).json();
  const fromSource = (broken.links ?? []).filter(
    (l: { from_page_id: string }) => l.from_page_id === srcId,
  );
  expect(fromSource.map((l: { to_path: string }) => l.to_path)).toEqual([`wikilink:${missing}`]);

  await api.dispose();
});
