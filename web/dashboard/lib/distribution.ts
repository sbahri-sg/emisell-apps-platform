// UI affordances only; the backend enforces the current distribution policy.
export const publicDistributionAllowed = (capability: string) =>
  capability === 'shipping/v1';
export const paymentBoundaryMessage =
  'Payment gateway dikelola internal Emisell melalui Settings → Payments, bukan aplikasi umum. Data payment historis tetap tersedia; pengajuan dan distribusi baru tidak diizinkan.';
