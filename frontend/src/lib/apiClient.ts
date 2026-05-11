const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:3001';

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('ca-token');
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

async function handleResponse(response: Response) {
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || response.statusText);
  }
  return response.json();
}

export const apiClient = {
  async get<T = unknown>(path: string): Promise<T> {
    const response = await fetch(`${BASE_URL}${path}`, { headers: getAuthHeaders() });
    return handleResponse(response);
  },

  async post<T = unknown>(path: string, body?: unknown): Promise<T> {
    const response = await fetch(`${BASE_URL}${path}`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: body ? JSON.stringify(body) : undefined,
    });
    return handleResponse(response);
  },

  async put<T = unknown>(path: string, body?: unknown): Promise<T> {
    const response = await fetch(`${BASE_URL}${path}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: body ? JSON.stringify(body) : undefined,
    });
    return handleResponse(response);
  },

  async delete<T = unknown>(path: string): Promise<T> {
    const response = await fetch(`${BASE_URL}${path}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },
};
