"use client";

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

  const handleClick = () => {
    reset();
    writeContract({
      address: contractAddress,
      abi,
      functionName,
      args,
    } as Parameters<typeof writeContract>[0]);
  };

  // Call onSuccess when confirmed
  if (isConfirmed && hash && onSuccess) {
    // Use setTimeout to avoid calling during render
    setTimeout(() => onSuccess(hash), 0);
  }

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
