"use client";

import { useAccount, useReadContract, useReadContracts } from "wagmi";
import { VAULT_REGISTRY_ADDRESS, vaultRegistryAbi } from "@/lib/contracts";
import { VaultCard } from "@/components/VaultCard";
import Link from "next/link";

export default function DashboardPage() {
  const { address, isConnected } = useAccount();

  // Get the next vault ID to know how many vaults exist
  const { data: nextVaultId } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "nextVaultId",
  });

  const totalVaults = nextVaultId ? Number(nextVaultId) - 1 : 0;

  // Build an array of vault IDs to query
  const vaultIds = Array.from({ length: totalVaults }, (_, i) => BigInt(i + 1));

  // Batch-read owner, state, balance, and timing for all vaults
  const { data: ownersData } = useReadContracts({
    contracts: vaultIds.map((id) => ({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "getVaultOwner" as const,
      args: [id],
    })),
  });

  const { data: statesData } = useReadContracts({
    contracts: vaultIds.map((id) => ({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "getVaultState" as const,
      args: [id],
    })),
  });

  const { data: balancesData } = useReadContracts({
    contracts: vaultIds.map((id) => ({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "getVaultBalance" as const,
      args: [id],
    })),
  });

  const { data: timingsData } = useReadContracts({
    contracts: vaultIds.map((id) => ({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "getVaultTiming" as const,
      args: [id],
    })),
  });

  // Filter vaults owned by the connected address
  const myVaults = vaultIds.filter((_, i) => {
    const owner = ownersData?.[i]?.result;
    return owner && address && (owner as string).toLowerCase() === address.toLowerCase();
  });

  const allVaultsLoaded = ownersData && statesData && balancesData && timingsData;

  return (
    <div className="page">
      <div className="container">
        {/* Hero */}
        <section className="animate-slide-up" style={{ textAlign: "center", marginBottom: "var(--space-3xl)", paddingTop: "var(--space-2xl)" }}>
          <h1 style={{ fontSize: "3rem", marginBottom: "var(--space-md)", lineHeight: 1.1 }}>
            <span style={{ display: "block", fontSize: "1rem", fontWeight: 500, color: "var(--text-tertiary)", letterSpacing: "0.1em", textTransform: "uppercase", marginBottom: "var(--space-sm)" }}>
              Non-custodial estate protocol
            </span>
            Your assets.{" "}
            <span style={{ background: "linear-gradient(135deg, var(--accent) 0%, hsl(280, 70%, 65%) 100%)", WebkitBackgroundClip: "text", WebkitTextFillColor: "transparent", backgroundClip: "text" }}>
              Your plan.
            </span>{" "}
            Your terms.
          </h1>
          <p style={{ maxWidth: 560, margin: "0 auto var(--space-xl)", fontSize: "1.1rem" }}>
            Private inheritance and business-continuity for XRP, BTC, and DOGE holders — powered by Flare&apos;s FAssets, FDC, and Confidential Compute.
          </p>

          {!isConnected && (
            <div style={{ display: "flex", justifyContent: "center", gap: "var(--space-md)" }}>
              <Link href="/observe" className="btn btn-secondary btn-lg">
                Observe a Vault
              </Link>
              <Link href="/trustee" className="btn btn-secondary btn-lg">
                I&apos;m a Trustee
              </Link>
            </div>
          )}
        </section>

        {/* Connected wallet: show vaults */}
        {isConnected && (
          <section className="animate-in">
            <div className="section-header">
              <h2>Your Vaults</h2>
              <Link href="/vault/create" className="btn btn-primary">
                + Create Vault
              </Link>
            </div>

            {!allVaultsLoaded && (
              <div className="card" style={{ textAlign: "center", padding: "var(--space-3xl)" }}>
                <span className="tx-spinner" style={{ width: 24, height: 24, display: "inline-block", borderWidth: 3 }} />
                <p style={{ marginTop: "var(--space-md)" }}>Loading vaults from Coston2…</p>
              </div>
            )}

            {allVaultsLoaded && myVaults.length === 0 && (
              <div className="card empty-state">
                <div className="empty-state-icon">🛡️</div>
                <h3>No vaults yet</h3>
                <p>
                  Create your first Continuity Vault to set up a private inheritance or business-continuity plan.
                </p>
                <Link href="/vault/create" className="btn btn-primary btn-lg">
                  Create Your First Vault
                </Link>
              </div>
            )}

            {allVaultsLoaded && myVaults.length > 0 && (
              <div className="grid-cards">
                {myVaults.map((vaultId, displayIdx) => {
                  const idx = Number(vaultId) - 1;
                  const state = statesData[idx]?.result as number ?? 0;
                  const balance = balancesData[idx]?.result as bigint ?? 0n;
                  const timing = timingsData[idx]?.result as readonly [bigint, bigint, bigint, bigint, bigint, bigint] | undefined;
                  const owner = ownersData[idx]?.result as string ?? "";

                  return (
                    <div key={vaultId.toString()} className={`stagger-${displayIdx + 1}`}>
                      <VaultCard
                        vaultId={vaultId}
                        stateIndex={state}
                        balance={balance}
                        windowDeadline={timing?.[1] ?? 0n}
                        checkInInterval={timing?.[2] ?? 0n}
                        owner={owner}
                      />
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        )}

        {/* Protocol stats */}
        {isConnected && totalVaults > 0 && allVaultsLoaded && (
          <section className="animate-in" style={{ marginTop: "var(--space-3xl)" }}>
            <h3 style={{ color: "var(--text-tertiary)", fontSize: "0.85rem", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: "var(--space-md)" }}>
              Protocol Overview
            </h3>
            <div className="grid-stats">
              <div className="card card-compact">
                <div className="stat">
                  <span className="stat-label">Total Vaults</span>
                  <span className="stat-value">{totalVaults}</span>
                </div>
              </div>
              <div className="card card-compact">
                <div className="stat">
                  <span className="stat-label">Your Vaults</span>
                  <span className="stat-value">{myVaults.length}</span>
                </div>
              </div>
              <div className="card card-compact">
                <div className="stat">
                  <span className="stat-label">Network</span>
                  <span className="stat-value" style={{ fontSize: "1.1rem" }}>Coston2</span>
                </div>
              </div>
            </div>
          </section>
        )}

        {/* How it works — brief overview for non-connected visitors */}
        {!isConnected && (
          <section className="animate-in" style={{ marginTop: "var(--space-2xl)" }}>
            <div className="grid-cards" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))" }}>
              {[
                { icon: "🔐", title: "Private Plans", desc: "Your beneficiaries, splits, and conditions stay sealed inside a TEE — invisible on-chain until execution." },
                { icon: "⚖️", title: "Multi-Signal Triggers", desc: "A missed check-in + independent trustee attestation. Never a single point of failure." },
                { icon: "🛡️", title: "Dispute Windows", desc: "Every trigger is provisional. Owner and guardian can halt before funds move — even after quorum." },
                { icon: "💰", title: "Native Settlement", desc: "Beneficiaries receive native XRP, not a wrapped token. Powered by FAssets redemption." },
              ].map((item) => (
                <div className="card" key={item.title}>
                  <div style={{ fontSize: "2rem", marginBottom: "var(--space-md)" }}>{item.icon}</div>
                  <h3 style={{ marginBottom: "var(--space-sm)" }}>{item.title}</h3>
                  <p style={{ fontSize: "0.9rem" }}>{item.desc}</p>
                </div>
              ))}
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
