"use client";

import Link from "next/link";
import { VaultStateBadge } from "./VaultStateBadge";
import { CountdownTimer } from "./CountdownTimer";
import { formatSeconds, truncateAddress } from "@/lib/constants";
import { formatEther } from "viem";

interface VaultCardProps {
  vaultId: bigint;
  stateIndex: number;
  balance: bigint;
  windowDeadline: bigint;
  checkInInterval: bigint;
  owner: string;
}

export function VaultCard({
  vaultId,
  stateIndex,
  balance,
  windowDeadline,
  checkInInterval,
  owner,
}: VaultCardProps) {
  const deadlineNum = Number(windowDeadline);
  const balanceFormatted = formatEther(balance);
  const intervalFormatted = formatSeconds(Number(checkInInterval));

  return (
    <Link href={`/vault/${vaultId.toString()}`} style={{ textDecoration: "none" }}>
      <div className="card card-interactive animate-in">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "var(--space-md)" }}>
          <div>
            <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.8rem", color: "var(--text-tertiary)" }}>
              VAULT
            </span>
            <h3 style={{ fontSize: "1.5rem", fontWeight: 700 }}>
              #{vaultId.toString()}
            </h3>
          </div>
          <VaultStateBadge stateIndex={stateIndex} />
        </div>

        <div className="grid-stats" style={{ marginBottom: "var(--space-md)" }}>
          <div className="stat">
            <span className="stat-label">Balance</span>
            <span className="stat-value" style={{ fontSize: "1.1rem" }}>
              {parseFloat(balanceFormatted).toFixed(4)}
              <span className="stat-unit"> FXRP</span>
            </span>
          </div>
          <div className="stat">
            <span className="stat-label">Check-in Interval</span>
            <span className="stat-value" style={{ fontSize: "1.1rem" }}>
              {intervalFormatted}
            </span>
          </div>
        </div>

        {stateIndex < 7 && deadlineNum > 0 && (
          <CountdownTimer deadline={deadlineNum} label="Next Deadline" />
        )}

        <div style={{ marginTop: "var(--space-md)", borderTop: "1px solid var(--border-subtle)", paddingTop: "var(--space-sm)" }}>
          <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.75rem", color: "var(--text-tertiary)" }}>
            Owner: {truncateAddress(owner)}
          </span>
        </div>
      </div>
    </Link>
  );
}
