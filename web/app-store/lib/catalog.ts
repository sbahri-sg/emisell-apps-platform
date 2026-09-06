import type { ScopeDeclaration } from "../../dashboard/lib/access-scopes.ts";
export type StoreApp = {
  id: string;
  appId: string;
  developerId: string;
  name: string;
  summary: string;
  description: string;
  version: string;
  capability: string;
  scopes: string[];
  accessScopes?: ScopeDeclaration;
  pricing: "free";
  installable: false;
};
export type StorePage = { apps: StoreApp[]; total: number; page: number; pageSize: number };
async function read<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch("/api/v1/store/apps" + path, {
    credentials: "omit",
    cache: "no-store",
    signal: signal
      ? AbortSignal.any([signal, AbortSignal.timeout(12000)])
      : AbortSignal.timeout(12000),
  });
  if (!response.ok)
    throw new Error(
      response.status === 404
        ? "Listing ini sudah tidak tersedia. Muat ulang katalog."
        : "Katalog belum dapat dimuat. Coba lagi beberapa saat.",
    );
  return response.json() as Promise<T>;
}
export function searchCatalog(
  search: string,
  capability: string,
  page: number,
  signal?: AbortSignal,
) {
  if (
    search.length > 120 ||
    !["", "payment/v1", "shipping/v1"].includes(capability) ||
    !Number.isInteger(page) ||
    page < 1 ||
    page > 500
  )
    throw new Error("Filter katalog tidak valid.");
  return read<StorePage>(
    "?" + new URLSearchParams({ search, capability, page: String(page) }),
    signal,
  );
}
export function readApp(id: string) {
  if (!/^[A-Za-z0-9_-]{1,100}$/.test(id)) throw new Error("ID aplikasi tidak valid.");
  return read<{ app: StoreApp }>("/" + id);
}
