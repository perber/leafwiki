const e2eBaseUrl = process.env.E2E_BASE_URL || 'http://localhost:8080';
export const e2eBasePath = new URL(e2eBaseUrl).pathname.replace(/\/$/, '');

export function toAppPath(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;

  if (!e2eBasePath) {
    return normalizedPath;
  }

  return `${e2eBasePath}${normalizedPath}`;
}
