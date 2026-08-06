export function Footer() {
  return (
    <footer className="footer">
      <div className="container footer-inner">
        <div className="footer-left">
          <span className="footer-logo">🛡️ Continuity Vault</span>
          <span className="footer-tagline">
            Non-custodial estate & business-continuity protocol
          </span>
        </div>

        <div className="footer-right">
          <span className="footer-built">
            Built on{" "}
            <a
              href="https://flare.network"
              target="_blank"
              rel="noopener noreferrer"
            >
              Flare
            </a>{" "}
            · FAssets · FDC · FCC
          </span>
          <span className="footer-network">Coston2 Testnet</span>
        </div>
      </div>

      <style jsx>{`
        .footer {
          border-top: 1px solid var(--border-subtle);
          padding: var(--space-xl) 0;
          margin-top: var(--space-3xl);
        }

        .footer-inner {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: var(--space-lg);
        }

        .footer-left {
          display: flex;
          flex-direction: column;
          gap: 4px;
        }

        .footer-logo {
          font-family: var(--font-heading);
          font-weight: 600;
          font-size: 0.9rem;
          color: var(--text-primary);
        }

        .footer-tagline {
          font-size: 0.78rem;
          color: var(--text-tertiary);
        }

        .footer-right {
          display: flex;
          flex-direction: column;
          align-items: flex-end;
          gap: 4px;
          text-align: right;
        }

        .footer-built {
          font-size: 0.78rem;
          color: var(--text-tertiary);
        }

        .footer-built a {
          color: var(--accent);
        }

        .footer-network {
          font-size: 0.72rem;
          color: var(--text-tertiary);
          opacity: 0.7;
        }

        @media (max-width: 640px) {
          .footer-inner {
            flex-direction: column;
            text-align: center;
          }

          .footer-right {
            align-items: center;
          }
        }
      `}</style>
    </footer>
  );
}
