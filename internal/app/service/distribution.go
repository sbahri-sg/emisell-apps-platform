package service

import "emisell.app/platform/internal/platform/fault"

// PublicCapability is the current authoring/distribution allowlist, not a wire
// schema. Historical payment manifests and signatures remain decodable.
const PublicCapability = "shipping/v1"

const PaymentBoundaryMessage = "Payment gateway dikelola internal Emisell melalui Settings → Payments, bukan aplikasi umum. Data payment historis tetap tersedia; pengajuan dan distribusi baru tidak diizinkan."

func PublicDistributionAllowed(capability string) bool { return capability == PublicCapability }

func ValidatePublicDistribution(capability string) error {
	if !PublicDistributionAllowed(capability) {
		return fault.Forbidden
	}
	return nil
}
