"use client";

import { STATE_DISPLAY, getStateName, type VaultStateName } from "@/lib/constants";

interface VaultStateBadgeProps {
  stateIndex: number;
  size?: "sm" | "md" | "lg";
}

export function VaultStateBadge({ stateIndex, size = "md" }: VaultStateBadgeProps) {
  const stateName = getStateName(stateIndex);
  const display = STATE_DISPLAY[stateName];

  const sizeStyles = {
    sm: { fontSize: "0.68rem", padding: "0.2rem 0.6rem" },
    md: { fontSize: "0.78rem", padding: "0.3rem 0.75rem" },
    lg: { fontSize: "0.88rem", padding: "0.4rem 1rem" },
  };

  return (
    <span
      className="badge"
      style={{
        background: display.color,
        color: display.textColor,
        boxShadow: display.glow !== "transparent" ? `0 0 12px ${display.glow}` : undefined,
        animation: display.pulse ? "glow-pulse 2.5s ease-in-out infinite" : undefined,
        ["--glow-color" as string]: display.glow,
        ...sizeStyles[size],
      }}
    >
      <span className="badge-dot" />
      {display.label}
    </span>
  );
}

export function getStateColor(stateIndex: number): string {
  return STATE_DISPLAY[getStateName(stateIndex)].textColor;
}
