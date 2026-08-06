"use client";

import { RainbowKitProvider, getDefaultConfig, darkTheme } from "@rainbow-me/rainbowkit";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiProvider, type Chain } from "wagmi";
import { type ReactNode, useState } from "react";
import "@rainbow-me/rainbowkit/styles.css";

/**
 * Coston2 — Flare's public testnet for app development.
 * Chain ID 114, native currency C2FLR.
 */
const coston2: Chain = {
  id: 114,
  name: "Coston2",
  nativeCurrency: {
    name: "Coston2 Flare",
    symbol: "C2FLR",
    decimals: 18,
  },
  rpcUrls: {
    default: {
      http: [process.env.NEXT_PUBLIC_COSTON2_RPC ?? "https://coston2-api.flare.network/ext/C/rpc"],
    },
  },
  blockExplorers: {
    default: {
      name: "Coston2 Explorer",
      url: process.env.NEXT_PUBLIC_EXPLORER_URL ?? "https://coston2-explorer.flare.network",
    },
  },
  testnet: true,
};

const wagmiConfig = getDefaultConfig({
  appName: "Continuity Vault",
  projectId: "continuity-vault-hackathon", // WalletConnect project ID — replace with real one for prod
  chains: [coston2],
  ssr: true,
});

export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <WagmiProvider config={wagmiConfig}>
      <QueryClientProvider client={queryClient}>
        <RainbowKitProvider
          theme={darkTheme({
            accentColor: "hsl(217, 91%, 60%)",
            accentColorForeground: "white",
            borderRadius: "medium",
            fontStack: "system",
            overlayBlur: "small",
          })}
        >
          {children}
        </RainbowKitProvider>
      </QueryClientProvider>
    </WagmiProvider>
  );
}
