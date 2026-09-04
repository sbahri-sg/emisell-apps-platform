import type { Metadata } from "next";
import { MerchantAppsView } from "../../merchant-portal";

export const metadata: Metadata = {
  title: "Connected apps",
  description: "Manage apps connected to an Emisell sandbox merchant.",
  robots: { index: false, follow: false },
};

export default function MerchantAppsPage() {
  return <MerchantAppsView />;
}
