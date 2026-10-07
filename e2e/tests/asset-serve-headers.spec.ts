import { expect, test } from '@playwright/test';
import { createPageViaApi, csrfHeader, loggedIn } from '../helpers/apiContext';

// Uploaded assets are served with nosniff; anything that is not an image,
// PDF, audio or video file is forced to download instead of rendering on the
// wiki origin.
test('uploaded assets: images render inline, svg and html download', async () => {
  const api = await loggedIn();
  const slug = `asset-headers-${Date.now()}`;
  const pageId = await createPageViaApi(api, { title: slug, slug, content: 'assets' });

  const upload = async (name: string, mimeType: string, body: string) => {
    const resp = await api.post(`/api/pages/${pageId}/assets`, {
      headers: await csrfHeader(api),
      multipart: { file: { name, mimeType, buffer: Buffer.from(body) } },
    });
    expect(resp.status(), `upload ${name}`).toBe(201);
    return (await resp.json()).file as string;
  };

  const cases = [
    { name: 'pic.png', mime: 'image/png', body: 'png-bytes', attachment: false },
    {
      name: 'drawing.svg',
      mime: 'image/svg+xml',
      body: '<svg xmlns="http://www.w3.org/2000/svg"/>',
      attachment: true,
    },
    { name: 'page.html', mime: 'text/html', body: '<script>alert(1)</script>', attachment: true },
  ];

  for (const c of cases) {
    const url = await upload(c.name, c.mime, c.body);
    const resp = await api.get(url);
    expect(resp.status(), `GET ${url}`).toBe(200);
    expect(resp.headers()['x-content-type-options'], c.name).toBe('nosniff');
    if (c.attachment) {
      expect(resp.headers()['content-disposition'], c.name).toBe('attachment');
    } else {
      expect(resp.headers()['content-disposition'], c.name).toBeUndefined();
    }
  }

  await api.dispose();
});
