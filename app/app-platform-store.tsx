"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { usePathname } from "next/navigation";
import type {
  CreateAppRequest,
  UpdateAppRequest,
} from "../lib/app-platform/contracts";
import type {
  App,
  OrganizationMembership,
  SessionActor,
} from "../lib/app-platform/domain";
import { apiErrorMessage, appPlatformClient } from "../lib/app-platform/client";

type AppPlatformStore = {
  apps: App[];
  loading: boolean;
  error: string | null;
  session: SessionActor | null;
  organizations: OrganizationMembership[];
  refreshApps: () => Promise<void>;
  createApp: (input: CreateAppRequest) => Promise<App>;
  updateApp: (appId: string, input: UpdateAppRequest) => Promise<App>;
  archiveApp: (appId: string) => Promise<void>;
  switchOrganization: (organizationId: string) => Promise<void>;
  logout: () => Promise<void>;
};

const StoreContext = createContext<AppPlatformStore | null>(null);

export function AppPlatformProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const merchantSurface =
    pathname === "/install" || pathname.startsWith("/merchant/");
  if (pathname === "/login" || pathname === '/admin/login' || merchantSurface) return <>{children}</>;

  const admin = pathname === '/admin' || pathname.startsWith('/admin/');
  return <ControlPlatformProvider key={admin ? 'admin' : 'developer'} admin={admin}>{children}</ControlPlatformProvider>;
}

function ControlPlatformProvider({ children, admin }: { children: React.ReactNode; admin: boolean }) {
  const [apps, setApps] = useState<App[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [session, setSession] = useState<SessionActor | null>(null);
  const [organizations, setOrganizations] = useState<OrganizationMembership[]>(
    [],
  );

  const loadApps = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const response = await appPlatformClient.listAllApps(signal);
      setApps(response);
    } catch (loadError) {
      if (loadError instanceof DOMException && loadError.name === "AbortError")
        return;
      setError(apiErrorMessage(loadError));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const bootstrap = async () => {
      try {
        if (admin) {
          setSession(await appPlatformClient.getAdminSession(controller.signal));
          return;
        }
        let currentSession = await appPlatformClient.getSession(
          controller.signal,
        );
        if (
          currentSession.authenticationMethod === "bearer" &&
          process.env.NEXT_PUBLIC_ENABLE_DEVELOPMENT_LOGIN === "true"
        ) {
          await appPlatformClient.developmentLogin();
          currentSession = await appPlatformClient.getSession(
            controller.signal,
          );
        }
        setSession(currentSession);
        const memberships = await appPlatformClient.listOrganizations(
          controller.signal,
        );
        setOrganizations(memberships);
        if (currentSession.organizationId)
          setApps(await appPlatformClient.listAllApps(controller.signal));
        else setApps([]);
      } catch (loadError) {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setError(apiErrorMessage(loadError));
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    };
    void bootstrap();
    return () => controller.abort();
  }, [admin]);

  const value = useMemo<AppPlatformStore>(
    () => ({
      apps,
      loading,
      error,
      session,
      organizations,
      refreshApps: () => loadApps(),
      createApp: async (input) => {
        const app = await appPlatformClient.createApp(input);
        setApps((current) => [
          app,
          ...current.filter((item) => item.id !== app.id),
        ]);
        return app;
      },
      updateApp: async (appId, input) => {
        const app = await appPlatformClient.updateApp(appId, input);
        setApps((current) =>
          current.map((item) => (item.id === app.id ? app : item)),
        );
        return app;
      },
      archiveApp: async (appId) => {
        await appPlatformClient.archiveApp(appId);
        setApps((current) =>
          current.map((item) =>
            item.id === appId
              ? {
                  ...item,
                  status: "archived",
                  revision: item.revision + 1,
                  updatedAt: new Date().toISOString(),
                }
              : item,
          ),
        );
      },
      switchOrganization: async (organizationId) => {
        await appPlatformClient.switchOrganization(organizationId);
        window.location.assign("/overview");
      },
      logout: async () => {
        if (admin) await appPlatformClient.adminLogout();
        else await appPlatformClient.logout();
        window.location.assign(admin ? '/admin/login' : '/login');
      },
    }),
    [admin, apps, error, loadApps, loading, organizations, session],
  );

  return (
    <StoreContext.Provider value={value}>{children}</StoreContext.Provider>
  );
}

export function useAppPlatform() {
  const store = useContext(StoreContext);
  if (!store)
    throw new Error("useAppPlatform must be used within AppPlatformProvider");
  return store;
}
