'use client';

import { ArrowRight, Eye, EyeOff, LoaderCircle, LockKeyhole, ShieldCheck } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { appPlatformClient, AppPlatformApiError } from '../../../lib/app-platform/client';
import './login.css';

function destination() {
  const next = new URLSearchParams(window.location.search).get('next') ?? '/admin';
  // Only console routes, never external URLs, auth endpoints, or backslashes.
  return /^\/admin(?:\/[a-z0-9-]+)*(?:\?[^\\\r\n#]*)?$/.test(next) && !next.startsWith('/admin/login') && !next.startsWith('/admin/docs/content') ? next : '/admin';
}

export default function AdminLoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [visible, setVisible] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const expired = useSearchParams().get('reason') === 'expired';

  useEffect(() => {
    const controller = new AbortController();
    void appPlatformClient.getAdminSession(controller.signal).then(() => window.location.replace(destination())).catch(() => {});
    return () => controller.abort();
  }, []);

  async function signIn(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (loading) return;
    setLoading(true);
    setError('');
    try {
      await appPlatformClient.adminLogin(email.trim(), password);
      setPassword('');
      window.location.replace(destination());
    } catch (failure) {
      setError(failure instanceof AppPlatformApiError && failure.status === 429
        ? 'Too many sign-in attempts. Please wait 15 minutes and try again.'
        : failure instanceof AppPlatformApiError && failure.status === 401
          ? 'Email or password is incorrect, or this account is unavailable.'
          : 'Sign-in is temporarily unavailable. Please try again shortly.');
      setPassword('');
      setLoading(false);
    }
  }

  return <main className="admin-login-page">
    <header className="admin-login-brand"><span className="admin-login-mark">E</span><div><strong>Emisell</strong><span>App Platform</span></div><span className="admin-login-label">Admin Console</span></header>
    <div className="admin-login-center">
      <section className="admin-login-card" aria-labelledby="admin-login-title">
        <div className="admin-login-icon"><LockKeyhole size={23} strokeWidth={1.7} /></div>
        <p className="admin-login-eyebrow">EMISELL INTERNAL</p>
        <h1 id="admin-login-title">Welcome back.</h1>
        <p className="admin-login-description">Sign in to manage developer access, organizations, and the app catalog.</p>
        {expired ? <p className="admin-login-message" role="status">Please sign in to continue. Your admin session may have expired.</p> : null}
        <form onSubmit={(event) => void signIn(event)} aria-busy={loading}>
          <label htmlFor="admin-email">Work email</label>
          <input id="admin-email" type="email" name="email" autoComplete="username" placeholder="you@emisell.com" value={email} onChange={(event) => setEmail(event.target.value)} required maxLength={254} disabled={loading} autoCapitalize="none" spellCheck={false} />
          <label htmlFor="admin-password">Password</label>
          <div className="admin-password-input"><input id="admin-password" type={visible ? 'text' : 'password'} name="password" autoComplete="current-password" placeholder="Enter your password" value={password} onChange={(event) => setPassword(event.target.value)} required maxLength={256} disabled={loading} aria-describedby={error ? 'admin-login-error' : undefined} /><button type="button" aria-label={visible ? 'Hide password' : 'Show password'} aria-pressed={visible} onClick={() => setVisible(!visible)} disabled={loading}>{visible ? <EyeOff size={18} /> : <Eye size={18} />}</button></div>
          {error ? <p id="admin-login-error" className="admin-login-error" role="alert">{error}</p> : null}
          <button type="submit" className="primary-button admin-login-submit" disabled={loading}>{loading ? <><LoaderCircle size={17} className="spin" />Signing in…</> : <>Sign in to Admin Console<ArrowRight size={17} /></>}</button>
        </form>
        <div className="admin-login-access"><ShieldCheck size={18} /><p><strong>Access by invitation only</strong>Admin accounts are created manually by Emisell. For access or a password reset, contact your platform administrator.</p></div>
      </section>
      <p className="admin-login-developer">Building an app? <a href="/login">Open the Developer Console <ArrowRight size={13} /></a></p>
    </div>
    <footer className="admin-login-footer"><span>Emisell App Platform</span><span>Internal administration · Restricted access</span></footer>
  </main>;
}
