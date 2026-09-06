export type ManagedShippingRelease = {
  id: string;
  status: 'submitted' | 'approved' | 'rejected' | 'signed' | 'suspended';
  manifest: {
    appId: string;
    name: string;
    version: string;
    capability: string;
    binding: { engine: 'api-kurir'; providerCode: 'emisell' };
  };
};
