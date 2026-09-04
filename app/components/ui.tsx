'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, CheckCircle2, Info, LoaderCircle, X } from 'lucide-react';

export function StatusBadge({ status }: { status: string }) {
  return <span className={`status ${status.toLowerCase().replace(' ', '-')}`}><i />{status}</span>;
}

export function Panel({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <section className={`panel ${className}`}>{children}</section>;
}

export function Toast({ message, tone = 'success', onClose }: { message: string; tone?: 'success' | 'info' | 'warning'; onClose: () => void }) {
  const Icon = tone === 'success' ? CheckCircle2 : tone === 'warning' ? AlertTriangle : Info;
  return <div className={`toast ${tone}`} role="status" aria-live="polite"><Icon size={17} /><span>{message}</span><button aria-label="Dismiss notification" onClick={onClose}><X size={14} /></button></div>;
}

export function ConfirmDialog({
  open,
  eyebrow = 'Please confirm',
  title,
  description,
  confirmLabel,
  tone = 'primary',
  loading = false,
  onClose,
  onConfirm,
}: {
  open: boolean;
  eyebrow?: string;
  title: string;
  description: string;
  confirmLabel: string;
  tone?: 'primary' | 'danger';
  loading?: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  if (!open) return null;
  return <div className="modal-backdrop" role="presentation" onMouseDown={onClose}><section className="modal confirm-modal" role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" onMouseDown={(event) => event.stopPropagation()}><div className="confirm-icon"><AlertTriangle size={20} /></div><div className="confirm-content"><p className="section-kicker">{eyebrow}</p><h2 id="confirm-title">{title}</h2><p>{description}</p></div><div className="modal-foot"><button className="secondary-button" onClick={onClose} disabled={loading}>Cancel</button><button className={tone === 'danger' ? 'danger-button' : 'primary-button'} onClick={onConfirm} disabled={loading}>{loading ? <LoaderCircle className="spin" size={15} /> : null}{loading ? 'Working…' : confirmLabel}</button></div></section></div>;
}

export function InlineNotice({ tone = 'info', title, children }: { tone?: 'info' | 'warning'; title: string; children: ReactNode }) {
  const Icon = tone === 'warning' ? AlertTriangle : Info;
  return <div className={`inline-notice ${tone}`}><Icon size={16} /><div><strong>{title}</strong><p>{children}</p></div></div>;
}
