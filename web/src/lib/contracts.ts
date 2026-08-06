/**
 * Continuity Vault — Contract ABIs & Addresses
 *
 * ABIs derived from the Solidity interfaces in contracts/src/interfaces/.
 * Addresses loaded from environment variables (set in .env.local).
 */

// ─── Addresses ───────────────────────────────────────────────────────

export const VAULT_REGISTRY_ADDRESS = (process.env.NEXT_PUBLIC_VAULT_REGISTRY_ADDRESS ?? "0x0000000000000000000000000000000000000001") as `0x${string}`;
export const FDC_VERIFIER_ADDRESS = (process.env.NEXT_PUBLIC_FDC_VERIFIER_ADDRESS ?? "0x0000000000000000000000000000000000000002") as `0x${string}`;
export const FASSETS_ROUTER_ADDRESS = (process.env.NEXT_PUBLIC_FASSETS_ROUTER_ADDRESS ?? "0x0000000000000000000000000000000000000003") as `0x${string}`;
export const FXRP_ADDRESS = (process.env.NEXT_PUBLIC_FXRP_ADDRESS ?? "0x0000000000000000000000000000000000000004") as `0x${string}`;

export const ATTESTATION_API_URL = process.env.NEXT_PUBLIC_ATTESTATION_API_URL ?? "http://localhost:3000";
export const ENCLAVE_API_URL = process.env.NEXT_PUBLIC_ENCLAVE_API_URL ?? "http://localhost:8080";
export const EXPLORER_URL = process.env.NEXT_PUBLIC_EXPLORER_URL ?? "https://coston2-explorer.flare.network";

// ─── VaultRegistry ABI ──────────────────────────────────────────────
// From contracts/src/interfaces/IVaultRegistry.sol + VaultRegistry.sol view helpers

export const vaultRegistryAbi = [
  // ── Write Functions ──
  {
    type: "function",
    name: "createVault",
    inputs: [
      { name: "planCommitmentHash", type: "bytes32" },
      { name: "fundingAsset", type: "address" },
      { name: "checkInIntervalSeconds", type: "uint256" },
      { name: "graceWindowSeconds", type: "uint256" },
      { name: "disputeWindowSeconds", type: "uint256" },
      { name: "finalWindowSeconds", type: "uint256" },
      { name: "guardianHaltKey", type: "address" },
    ],
    outputs: [{ name: "vaultId", type: "uint256" }],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "checkIn",
    inputs: [
      { name: "vaultId", type: "uint256" },
      { name: "signature", type: "bytes" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "fundVault",
    inputs: [
      { name: "vaultId", type: "uint256" },
      { name: "amount", type: "uint256" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "anchorLegalDoc",
    inputs: [
      { name: "vaultId", type: "uint256" },
      { name: "legalDocHash", type: "bytes32" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "requestAttestation",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "submitQuorumResult",
    inputs: [
      { name: "vaultId", type: "uint256" },
      { name: "quorumMet", type: "bool" },
      { name: "fceSignature", type: "bytes" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "guardianHalt",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "finalizeDisputeWindow",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "finalizeFinalWindow",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "cancelVault",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "markWarning",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "ownerOverride",
    inputs: [
      { name: "vaultId", type: "uint256" },
      { name: "signature", type: "bytes" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  // ── Read Functions ──
  {
    type: "function",
    name: "nextVaultId",
    inputs: [],
    outputs: [{ name: "", type: "uint256" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "getVaultState",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [{ name: "", type: "uint8" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "getVaultBalance",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [{ name: "", type: "uint256" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "getVaultOwner",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [{ name: "", type: "address" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "getVaultConfig",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [
      { name: "owner", type: "address" },
      { name: "planCommitmentHash", type: "bytes32" },
      { name: "fundingAsset", type: "address" },
      { name: "guardianHaltKey", type: "address" },
      { name: "legalDocHash", type: "bytes32" },
    ],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "getVaultTiming",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [
      { name: "lastCheckIn", type: "uint256" },
      { name: "windowDeadline", type: "uint256" },
      { name: "checkInInterval", type: "uint256" },
      { name: "graceWindow", type: "uint256" },
      { name: "disputeWindow", type: "uint256" },
      { name: "finalWindow", type: "uint256" },
    ],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "isCheckInMissed",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [{ name: "", type: "bool" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "vaultAttestationCount",
    inputs: [{ name: "vaultId", type: "uint256" }],
    outputs: [{ name: "", type: "uint256" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "quorumThreshold",
    inputs: [],
    outputs: [{ name: "", type: "uint256" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "enclaveOracle",
    inputs: [],
    outputs: [{ name: "", type: "address" }],
    stateMutability: "view",
  },
  // ── Events ──
  {
    type: "event",
    name: "VaultCreated",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "owner", type: "address", indexed: true },
      { name: "planCommitmentHash", type: "bytes32", indexed: false },
    ],
  },
  {
    type: "event",
    name: "CheckIn",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "nextDeadline", type: "uint256", indexed: false },
    ],
  },
  {
    type: "event",
    name: "VaultFunded",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "amount", type: "uint256", indexed: false },
      { name: "totalBalance", type: "uint256", indexed: false },
    ],
  },
  {
    type: "event",
    name: "StateTransition",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "from", type: "uint8", indexed: false },
      { name: "to", type: "uint8", indexed: false },
    ],
  },
  {
    type: "event",
    name: "LegalDocAnchored",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "legalDocHash", type: "bytes32", indexed: false },
    ],
  },
  {
    type: "event",
    name: "AttestationRequested",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
    ],
  },
  {
    type: "event",
    name: "QuorumResultSubmitted",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "quorumMet", type: "bool", indexed: false },
    ],
  },
  {
    type: "event",
    name: "GuardianHalt",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "guardian", type: "address", indexed: false },
    ],
  },
  {
    type: "event",
    name: "TrancheReleased",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
      { name: "tranche", type: "uint256", indexed: false },
      { name: "amount", type: "uint256", indexed: false },
    ],
  },
  {
    type: "event",
    name: "VaultFullyReleased",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
    ],
  },
  {
    type: "event",
    name: "VaultCancelled",
    inputs: [
      { name: "vaultId", type: "uint256", indexed: true },
    ],
  },
] as const;

// ─── ERC-20 ABI (minimal for approve + transferFrom) ────────────────

export const erc20Abi = [
  {
    type: "function",
    name: "approve",
    inputs: [
      { name: "spender", type: "address" },
      { name: "amount", type: "uint256" },
    ],
    outputs: [{ name: "", type: "bool" }],
    stateMutability: "nonpayable",
  },
  {
    type: "function",
    name: "allowance",
    inputs: [
      { name: "owner", type: "address" },
      { name: "spender", type: "address" },
    ],
    outputs: [{ name: "", type: "uint256" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "balanceOf",
    inputs: [{ name: "account", type: "address" }],
    outputs: [{ name: "", type: "uint256" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "symbol",
    inputs: [],
    outputs: [{ name: "", type: "string" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "decimals",
    inputs: [],
    outputs: [{ name: "", type: "uint8" }],
    stateMutability: "view",
  },
] as const;
