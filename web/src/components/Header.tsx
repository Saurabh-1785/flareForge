"use client";

import Link from "next/link";
import { ConnectButton } from "@rainbow-me/rainbowkit";

export function Header() {
  return (
    <header className="header">
      <div className="container header-inner">
        <Link href="/" className="header-logo">
          <span className="header-logo-icon">🛡️</span>
          <span className="header-logo-text">Continuity Vault</span>
        </Link>

        <nav className="header-nav">
          <Link href="/" className="header-nav-link">
            Dashboard
          </Link>
          <Link href="/vault/create" className="header-nav-link">
            Create Vault
          </Link>
          <Link href="/trustee" className="header-nav-link">
            Trustee
          </Link>
          <Link href="/observe" className="header-nav-link">
            Observer
          </Link>
        </nav>

        <div className="header-wallet">
          <ConnectButton
            showBalance={true}
            chainStatus="icon"
            accountStatus="avatar"
          />
        </div>
      </div>

      <style jsx>{`
        .header {
          position: sticky;
          top: 0;
          z-index: 100;
          background: hsla(222, 30%, 6%, 0.85);
          backdrop-filter: blur(20px);
          -webkit-backdrop-filter: blur(20px);
          border-bottom: 1px solid var(--border-subtle);
          height: var(--header-height);
          display: flex;
          align-items: center;
        }

        .header-inner {
          display: flex;
          align-items: center;
          justify-content: space-between;
          width: 100%;
          gap: var(--space-lg);
        }

        .header-logo {
          display: flex;
          align-items: center;
          gap: var(--space-sm);
          text-decoration: none;
          color: var(--text-primary);
          flex-shrink: 0;
        }

        .header-logo-icon {
          font-size: 1.4rem;
        }

        .header-logo-text {
          font-family: var(--font-heading);
          font-size: 1.15rem;
          font-weight: 700;
          letter-spacing: -0.02em;
          background: linear-gradient(135deg, var(--text-primary) 0%, var(--accent) 100%);
          -webkit-background-clip: text;
          -webkit-text-fill-color: transparent;
          background-clip: text;
        }

        .header-nav {
          display: flex;
          align-items: center;
          gap: var(--space-xs);
        }

        .header-nav-link {
          font-size: 0.85rem;
          font-weight: 500;
          color: var(--text-secondary);
          padding: 0.4rem 0.8rem;
          border-radius: var(--radius-sm);
          transition: all var(--transition-fast);
          text-decoration: none;
        }

        .header-nav-link:hover {
          color: var(--text-primary);
          background: var(--bg-elevated);
        }

        .header-wallet {
          flex-shrink: 0;
        }

        @media (max-width: 768px) {
          .header-nav {
            display: none;
          }
        }
      `}</style>
    </header>
  );
}
