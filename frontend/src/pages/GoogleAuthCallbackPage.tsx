import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/store/useAuthStore';
import type { AuthUser } from '@/store/useAuthStore';
import { getApiBaseUrl } from '@/lib/config';

/**
 * Handles the Google OAuth callback.
 *
 * The backend redirects here with ?code=<opaque-one-time-code>.
 * We POST the code to /api/auth/google/exchange to get the JWT —
 * the JWT itself never touches the URL.
 */
export function GoogleAuthCallbackPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { loginWithGoogleToken, logout } = useAuthStore();
  const exchanged = useRef(false); // Prevent double-fire in StrictMode

  useEffect(() => {
    if (exchanged.current) return;
    exchanged.current = true;

    const code = searchParams.get('code');
    const error = searchParams.get('error');

    if (error || !code) {
      navigate('/login?error=' + encodeURIComponent(error ?? 'google_callback_missing'), {
        replace: true,
      });
      return;
    }

    (async () => {
      try {
        const res = await fetch(`${getApiBaseUrl()}/api/auth/google/exchange`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include', // needed for the refresh_token cookie
          body: JSON.stringify({ code }),
        });

        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          const msg = (data as { error?: string }).error ?? 'exchange_failed';
          navigate('/login?error=' + encodeURIComponent(msg), { replace: true });
          return;
        }

        const data = (await res.json()) as { access_token: string; user: AuthUser };
        loginWithGoogleToken(data.access_token, data.user);
        navigate('/agents', { replace: true });
      } catch {
        logout();
        navigate('/login?error=network_error', { replace: true });
      }
    })();
  }, [searchParams, loginWithGoogleToken, logout, navigate]);

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        background: 'var(--color-background)',
        color: 'var(--color-text-secondary)',
        fontFamily: 'Satoshi, sans-serif',
        fontSize: '0.9375rem',
      }}
    >
      Signing you in with Google…
    </div>
  );
}
