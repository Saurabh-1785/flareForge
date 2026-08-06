/**
 * Continuity Vault — Constants
 *
 * State names, colors, demo presets, and shared config.
 */

// ─── Vault States ────────────────────────────────────────────────────
// Enum values matching VaultRegistry.sol's VaultState enum (0-indexed)

export const VAULT_STATES = [
  "ACTIVE",
  "WARNING",
  "QUORUM_PENDING",
  "DISPUTE_WINDOW",
  "SLASHING_REVIEW",
  "TRANCHE_1_RELEASED",
  "FINAL_WINDOW",
  "FULLY_RELEASED",
  "CLOSED",
] as const;

export type VaultStateName = (typeof VAULT_STATES)[number];

export function getStateName(stateIndex: number): VaultStateName {
  return VAULT_STATES[stateIndex] ?? "CLOSED";
}

// ─── State Display Config ────────────────────────────────────────────

export interface StateDisplay {
  label: string;
  color: string;       // CSS color for badge bg
  textColor: string;   // CSS color for badge text
  glow: string;        // Box-shadow glow color
  description: string;
  pulse: boolean;      // Whether the badge should animate
}

export const STATE_DISPLAY: Record<VaultStateName, StateDisplay> = {
  ACTIVE: {
    label: "Active",
    color: "hsl(152, 68%, 18%)",
    textColor: "hsl(152, 76%, 62%)",
    glow: "hsla(152, 76%, 50%, 0.25)",
    description: "Vault is live. Owner checks in on schedule.",
    pulse: false,
  },
  WARNING: {
    label: "Warning",
    color: "hsl(38, 80%, 16%)",
    textColor: "hsl(38, 92%, 60%)",
    glow: "hsla(38, 92%, 50%, 0.3)",
    description: "Check-in deadline missed. Grace period running.",
    pulse: true,
  },
  QUORUM_PENDING: {
    label: "Quorum Pending",
    color: "hsl(24, 80%, 16%)",
    textColor: "hsl(24, 95%, 60%)",
    glow: "hsla(24, 95%, 50%, 0.3)",
    description: "Awaiting independent attestation signals.",
    pulse: true,
  },
  DISPUTE_WINDOW: {
    label: "Dispute Window",
    color: "hsl(0, 70%, 16%)",
    textColor: "hsl(0, 82%, 62%)",
    glow: "hsla(0, 82%, 50%, 0.3)",
    description: "Quorum met. Owner or guardian can still halt.",
    pulse: true,
  },
  SLASHING_REVIEW: {
    label: "Slashing Review",
    color: "hsl(280, 60%, 16%)",
    textColor: "hsl(280, 70%, 65%)",
    glow: "hsla(280, 70%, 50%, 0.25)",
    description: "False-attestation challenge filed (Phase 2).",
    pulse: false,
  },
  TRANCHE_1_RELEASED: {
    label: "Tranche 1 Released",
    color: "hsl(210, 70%, 16%)",
    textColor: "hsl(210, 80%, 65%)",
    glow: "hsla(210, 80%, 50%, 0.25)",
    description: "First 50% of funds released.",
    pulse: false,
  },
  FINAL_WINDOW: {
    label: "Final Window",
    color: "hsl(350, 65%, 16%)",
    textColor: "hsl(350, 75%, 62%)",
    glow: "hsla(350, 75%, 50%, 0.3)",
    description: "Last chance for guardian halt before full release.",
    pulse: true,
  },
  FULLY_RELEASED: {
    label: "Fully Released",
    color: "hsl(210, 15%, 16%)",
    textColor: "hsl(210, 15%, 55%)",
    glow: "transparent",
    description: "All funds have been released to beneficiaries.",
    pulse: false,
  },
  CLOSED: {
    label: "Closed",
    color: "hsl(210, 10%, 14%)",
    textColor: "hsl(210, 10%, 45%)",
    glow: "transparent",
    description: "Vault cancelled by owner.",
    pulse: false,
  },
};

// ─── Lifecycle Steps (for the timeline stepper) ─────────────────────

export const LIFECYCLE_STEPS: VaultStateName[] = [
  "ACTIVE",
  "WARNING",
  "QUORUM_PENDING",
  "DISPUTE_WINDOW",
  "TRANCHE_1_RELEASED",
  "FINAL_WINDOW",
  "FULLY_RELEASED",
];

// ─── Demo Presets ────────────────────────────────────────────────────
// Short timers for live demo — full lifecycle in ~8 minutes

export const DEMO_PRESETS = {
  checkInInterval: 120,   // 2 minutes
  graceWindow: 60,        // 1 minute
  disputeWindow: 90,      // 1.5 minutes
  finalWindow: 60,        // 1 minute
};

// ─── Attestation Types ──────────────────────────────────────────────

export const ATTESTATION_TYPES = [
  { value: "DEATH", label: "Death", description: "The vault holder has passed away" },
  { value: "INCAPACITATION", label: "Incapacitation", description: "The vault holder is permanently incapacitated" },
  { value: "KEY_PERSON_DEPARTURE", label: "Key Person Departure", description: "The key person has departed the organization" },
] as const;

// ─── Formatting Helpers ─────────────────────────────────────────────

export function formatSeconds(totalSeconds: number): string {
  if (totalSeconds <= 0) return "0s";
  const d = Math.floor(totalSeconds / 86400);
  const h = Math.floor((totalSeconds % 86400) / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = totalSeconds % 60;

  const parts: string[] = [];
  if (d > 0) parts.push(`${d}d`);
  if (h > 0) parts.push(`${h}h`);
  if (m > 0) parts.push(`${m}m`);
  if (s > 0 || parts.length === 0) parts.push(`${s}s`);
  return parts.join(" ");
}

export function truncateAddress(address: string): string {
  if (!address || address.length < 10) return address;
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

export function explorerTxUrl(txHash: string): string {
  const base = process.env.NEXT_PUBLIC_EXPLORER_URL ?? "https://coston2-explorer.flare.network";
  return `${base}/tx/${txHash}`;
}

export function explorerAddressUrl(address: string): string {
  const base = process.env.NEXT_PUBLIC_EXPLORER_URL ?? "https://coston2-explorer.flare.network";
  return `${base}/address/${address}`;
}
