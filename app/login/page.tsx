'use client';

import { ArrowRight, Boxes, LoaderCircle, ShieldCheck } from 'lucide-react';
import { useState } from 'react';
import { apiErrorMessage, appPlatformClient } from '../../lib/app-platform/client';

export default function LoginPage() {
  const developmentLogin = process.env.NEXT_PUBLIC_ENABLE_DEVELOPMENT_LOGIN === 'true';
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const signInForDevelopment = async () => {
    setLoading(true);
    setError(null);
    try {
      await appPlatformClient.developmentLogin();
      window.location.assign('/overview');
    } catch (loginError) {
      setError(apiErrorMessage(loginError));
      setLoading(false);
    }
  };

  return <main className="login-page">
    <section className="login-card">
      <div className="login-brand"><span><Boxes size={20} /></span><div><strong>Emisell</strong><small>App Platform</small></div></div>
      <div className="login-copy"><p className="eyebrow">Developer identity</p><h1>Sign in to your workspace</h1><p>Access apps, versions, credentials, webhooks, and the organizations where you are a member.</p></div>
      {developmentLogin ? <button className="primary-button login-action" disabled={loading} onClick={() => void signInForDevelopment()}>{loading ? <LoaderCircle className="spin" size={16} /> : <ShieldCheck size={16} />}{loading ? 'Starting secure session…' : 'Continue in development'}{!loading ? <ArrowRight size={15} /> : null}</button> : <a className="primary-button login-action" href={appPlatformClient.loginURL('/overview')}><ShieldCheck size={16} />Continue with identity provider<ArrowRight size={15} /></a>}
      {error ? <p className="login-error" role="alert">{error}</p> : null}
      <div className="login-note"><ShieldCheck size={15} /><span>Your password stays with the configured identity provider. Emisell stores only a short-lived server-side session.</span></div>
    </section>
  </main>;
}
