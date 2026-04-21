/**
 * Minimal imageCache stub for Orchid.
 * The monolith cached images in IndexedDB for chat replay.
 * Agent-builder doesn't need that — uploads go straight to the backend.
 */

interface ImageMeta {
  filename: string;
  mime_type: string;
  size: number;
  url: string;
}

// no-op — images are served from the backend, not cached locally
export async function storeImage(_fileId: string, _blob: Blob, _meta: ImageMeta): Promise<void> {
  return Promise.resolve();
}

export async function getImage(_fileId: string): Promise<Blob | null> {
  return null;
}
