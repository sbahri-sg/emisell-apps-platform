import type { Metadata } from "next";
import { MerchantInstallView } from "../merchant-portal";

export const metadata: Metadata = {
  title: "Install app",
  description: "Review and approve an Emisell sandbox app installation.",
  robots: { index: false, follow: false },
};

export default function InstallPage() {
  return <MerchantInstallView />;
}
