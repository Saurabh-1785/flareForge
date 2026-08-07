"use client";

import { useEffect, useRef } from "react";
import { useWriteContract, useWaitForTransactionReceipt } from "wagmi";
import { type Abi } from "viem";
import { explorerTxUrl } from "@/lib/constants";

interface TransactionButtonProps {
  contractAddress: `0x${string}`;
  abi: Abi;
  functionName: string;
  args: unknown[];
  label: string;
  className?: string;
  disabled?: boolean;
  onSuccess?: (hash: string) => void;
}

export function TransactionButton({
  contractAddress,
  abi,
  functionName,
  args,
  label,
  className = "btn btn-primary",
  disabled = false,
  onSuccess,
}: TransactionButtonProps) {
  const {
    data: hash,
    writeContract,
    isPending: isWriting,
    error: writeError,
    reset,
  } = useWriteContract();

  const {
    isLoading: isConfirming,
    isSuccess: isConfirmed,
    error: confirmError,
  } = useWaitForTransactionReceipt({
    hash,
  });

  // Track whether onSuccess has already been called for this tx hash
  const calledRef = useRef<string | null>(null);

  const handleClick = () => {
    calledRef.current = null;
    reset();
    writeContract({
      address: contractAddress,
      abi,
      functionName,
      args,
    } as Parameters<typeof writeContract>[0]);
  };

  // Fire onSuccess exactly once per confirmed transaction
  useEffect(() => {
    if (isConfirmed && hash && onSuccess && calledRef.current !== hash) {
      calledRef.current = hash;
      onSuccess(hash);
    }
  }, [isConfirmed, hash, onSuccess]);

  const isLoading = isWriting || isConfirming;
  const error = writeError || confirmError;

  return (
    <div>
      <button
        className={className}
        onClick={handleClick}
        disabled={disabled || isLoading}
      >
        {isWriting && (
          <>
            <span className="tx-spinner" />
            Confirm in wallet…
          </>
        )}
        {isConfirming && (
          <>
            <span className="tx-spinner" />
            Confirming…
          </>
        )}
        {!isLoading && label}
      </button>

      {isConfirmed && hash && (
        <div className="tx-status success" style={{ marginTop: "var(--space-sm)" }}>
          ✓ Confirmed —{" "}
          <a
            href={explorerTxUrl(hash)}
            target="_blank"
            rel="noopener noreferrer"
            style={{ color: "inherit", textDecoration: "underline" }}
          >
            View on explorer
          </a>
        </div>
      )}

      {error && (
        <div className="tx-status error" style={{ marginTop: "var(--space-sm)" }}>
          ✕ {error.message?.slice(0, 120) ?? "Transaction failed"}
        </div>
      )}
    </div>
  );
}
