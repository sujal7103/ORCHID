import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { getApiBaseUrl } from '@/lib/config';
import { useAuthStore } from '@/store/useAuthStore';
import './LoginPage.css';

type Mode = 'login' | 'register';

export function LoginPage() {
  const navigate = useNavigate();
  const { login, register, isLoading, error, clearError } = useAuthStore();
  const [mode, setMode] = useState<Mode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [searchParams] = useSearchParams();
  const googleError = searchParams.get('error');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    clearError();
    try {
      if (mode === 'login') {
        await login(email, password);
      } else {
        await register(email, password, name);
      }
      navigate('/agents', { replace: true });
    } catch {
      // error is set in the store
    }
  }

  function toggleMode() {
    clearError();
    setMode(m => (m === 'login' ? 'register' : 'login'));
  }

  return (
    <div className="onboarding-container">
      {/* Left side: Welcome image (60%) */}
      <div className="onboarding-left">
        <div className="onboarding-image-container">
          <img src="/Welcome-page.png" alt="Orchid" className="onboarding-image" />
        </div>
      </div>

      {/* Right side: Auth form (40%) */}
      <div className="onboarding-auth">
        <div className="auth-form-wrapper">
          {/* Logo & tagline */}
          <div className="auth-header">
            <div className="auth-logo">
              <img src="/favicon.svg" alt="Orchid" className="auth-logo-icon" />
              <h1 className="auth-title">Orchid</h1>
            </div>
            <p className="auth-subtitle">
              Visual AI workflow builder
            </p>
          </div>

          {/* Auth card */}
          <div className="auth-card">
            <h2 className="auth-card-title">
              {mode === 'login' ? 'Welcome back' : 'Create your account'}
            </h2>
            <p className="auth-card-desc">
              {mode === 'login'
                ? 'Sign in to continue building workflows'
                : 'Get started with Orchid for free'}
            </p>

            {error && (
              <div className="auth-error">
                {error}
              </div>
            )}

            {googleError && !error && (
              <div className="auth-error">
                {googleError === 'google_auth_failed'
                  ? 'Google sign-in failed. Please try again.'
                  : googleError === 'state_mismatch' || googleError === 'state_cookie_mismatch'
                  ? 'Security check failed. Please try again.'
                  : googleError === 'exchange_failed' || googleError === 'invalid_or_expired_exchange_code'
                  ? 'Sign-in session expired. Please try again.'
                  : 'Google sign-in failed. Please try again or use email/password.'}
              </div>
            )}

            <form onSubmit={handleSubmit} className="auth-form">
              {mode === 'register' && (
                <div className="auth-field">
                  <label>Name</label>
                  <input
                    type="text"
                    value={name}
                    onChange={e => setName(e.target.value)}
                    required
                    placeholder="Your name"
                  />
                </div>
              )}

              <div className="auth-field">
                <label>Email</label>
                <input
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  required
                  placeholder="you@example.com"
                />
              </div>

              <div className="auth-field">
                <label>Password</label>
                <input
                  type="password"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  required
                  minLength={8}
                  placeholder="••••••••"
                />
              </div>

              <button
                type="submit"
                disabled={isLoading}
                className="auth-submit"
              >
                {isLoading
                  ? mode === 'login' ? 'Signing in…' : 'Creating account…'
                  : mode === 'login' ? 'Sign in' : 'Create account'}
              </button>
            </form>

            {/* ── Divider ── */}
            <div className="auth-divider">
              <span>or</span>
            </div>

            {/* ── Google Sign-In ── */}
            <a
              href={`${getApiBaseUrl()}/api/auth/google`}
              className="auth-google-btn"
            >
              <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
                <path d="M16.51 8H8.98v3h4.3c-.18 1-.74 1.48-1.6 2.04v2.01h2.6a7.8 7.8 0 0 0 2.38-5.88c0-.57-.05-.66-.15-1.18z" fill="#4285F4"/>
                <path d="M8.98 17c2.16 0 3.97-.72 5.3-1.94l-2.6-2.01c-.72.48-1.63.76-2.7.76-2.08 0-3.84-1.4-4.47-3.29H1.87v2.07A8 8 0 0 0 8.98 17z" fill="#34A853"/>
                <path d="M4.51 10.52A4.8 4.8 0 0 1 4.26 9c0-.53.09-1.04.25-1.52V5.41H1.87A8 8 0 0 0 .98 9c0 1.29.31 2.51.89 3.59l2.64-2.07z" fill="#FBBC05"/>
                <path d="M8.98 3.58c1.17 0 2.23.4 3.06 1.2l2.3-2.3A8 8 0 0 0 8.98 1 8 8 0 0 0 1.87 5.41l2.64 2.07C5.14 5 6.9 3.58 8.98 3.58z" fill="#EA4335"/>
              </svg>
              Continue with Google
            </a>

            <p className="auth-toggle">
              {mode === 'login' ? "Don't have an account?" : 'Already have an account?'}{' '}
              <button onClick={toggleMode}>
                {mode === 'login' ? 'Register' : 'Sign in'}
              </button>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
