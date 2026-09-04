declare module 'virtual:emisell-documentation' {
  export const platform: Record<string, unknown>;
  export const provider: Record<string, unknown>;
  export const resource: Record<string, unknown>;
  export const resourceBlueprint: Record<string, unknown>;
  export const shippingProvider: Record<string, unknown>;
}

declare module 'virtual:emisell-developer-guide' {
  export const developerGuide: import('./developer-types').DeveloperGuide;
}
