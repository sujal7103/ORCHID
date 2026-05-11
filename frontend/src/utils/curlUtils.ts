export interface ParsedCurl {
  method: string;
  url: string;
  headers: Record<string, string>;
  body?: string;
}

export function parseCurl(curlCommand: string): ParsedCurl {
  const result: ParsedCurl = { method: 'GET', url: '', headers: {} };

  const parts = curlCommand.replace(/\\\n/g, ' ').split(/\s+/);

  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    if (part === '-X' || part === '--request') {
      result.method = parts[++i]?.toUpperCase() || 'GET';
    } else if (part === '-H' || part === '--header') {
      const header = parts[++i]?.replace(/^['"]|['"]$/g, '') || '';
      const colonIdx = header.indexOf(':');
      if (colonIdx > 0) {
        result.headers[header.slice(0, colonIdx).trim()] = header.slice(colonIdx + 1).trim();
      }
    } else if (part === '-d' || part === '--data' || part === '--data-raw') {
      result.body = parts[++i]?.replace(/^['"]|['"]$/g, '') || '';
    } else if (part.startsWith('http://') || part.startsWith('https://')) {
      result.url = part.replace(/^['"]|['"]$/g, '');
    }
  }

  if (result.body && result.method === 'GET') result.method = 'POST';
  return result;
}

export function exportCurl(
  method: string,
  url: string,
  headers: Record<string, string>,
  body?: string,
): string {
  let cmd = `curl -X ${method} "${url}"`;
  for (const [key, value] of Object.entries(headers)) {
    cmd += ` \\\n  -H "${key}: ${value}"`;
  }
  if (body) {
    cmd += ` \\\n  -d '${body}'`;
  }
  return cmd;
}

export function parseQueryParamsFromUrl(url: string): Record<string, string> {
  try {
    const urlObj = new URL(url);
    const params: Record<string, string> = {};
    urlObj.searchParams.forEach((value, key) => { params[key] = value; });
    return params;
  } catch {
    return {};
  }
}

export function buildUrlWithParams(baseUrl: string, params: Record<string, string>): string {
  try {
    const url = new URL(baseUrl);
    Object.entries(params).forEach(([key, value]) => {
      if (value) url.searchParams.set(key, value);
    });
    return url.toString();
  } catch {
    return baseUrl;
  }
}
