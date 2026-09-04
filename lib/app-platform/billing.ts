export type AppPlan = {
  id: string; appId: string; appName: string; name: string; description: string;
  amountMinor: number; currency: 'IDR' | 'USD'; interval: 'free' | 'monthly';
  features: string[]; status: 'active' | 'archived'; createdAt: string;
};

export type CreateAppPlan = Pick<AppPlan, 'name' | 'description' | 'amountMinor' | 'currency' | 'interval' | 'features'>;

export type AppSubscriptionQuote = {
  id: string; installationId: string; plan: AppPlan; amountMinor: number;
  periodStart: string; periodEnd: string; expiresAt: string; accountRevision: number; termsVersion: string;
};

export type AppSubscription = {
  id: string; installationId: string; quoteId: string; plan: AppPlan;
  status: 'active' | 'pending_payment' | 'past_due' | 'cancelled';
  approvedBy: string; approvedAt: string; termsVersion: string;
  paidThrough: string | null; cancelledAt: string | null; cancellationReason?: string;
};

export type InstallationBilling = {
  plans: AppPlan[]; subscription: AppSubscription | null;
  paidAccess: boolean; paidBillingAvailable: boolean; test: boolean;
};

export function appPrice(amountMinor: number, currency: AppPlan['currency']) {
  return new Intl.NumberFormat('id-ID', { style: 'currency', currency, maximumFractionDigits: currency === 'IDR' ? 0 : 2 }).format(amountMinor / (currency === 'USD' ? 100 : 1));
}
