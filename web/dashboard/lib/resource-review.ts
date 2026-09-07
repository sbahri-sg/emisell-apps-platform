export function resourceReviewActions(surface: string, role: string, status: string): string[] {
  if (surface !== 'admin') return [];
  if (status === 'submitted' && ['administrator', 'reviewer'].includes(role)) return ['approved', 'rejected'];
  if (role !== 'administrator') return [];
  if (status === 'approved') return ['signed', 'suspended'];
  return status === 'signed' ? ['suspended'] : [];
}
