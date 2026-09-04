import type { Metadata } from "next";
import { MerchantStoreView } from "../../merchant-portal";

export const metadata: Metadata = {
  title: "App Store · Emisell",
  description: "Discover reviewed apps for your Emisell store.",
};

export default function MerchantAppStorePage() {
  return <MerchantStoreView />;
}
