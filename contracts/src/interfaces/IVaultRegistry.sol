// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

/**
 * @title IVaultRegistry
 * @notice Interface for the Continuity Vault state machine.
 * @dev The canonical lifecycle contract — holds plan commitment hashes only,
 *      never the plan itself. All windows are configurable for demo vs production.
 */
interface IVaultRegistry {
    // ─── Vault States ────────────────────────────────────────────────────
    // Exactly matching continuity-vault-architecture.md's state diagram.
    enum VaultState {
        ACTIVE,
        WARNING,
        QUORUM_PENDING,
        DISPUTE_WINDOW,
        SLASHING_REVIEW,       // Phase 2 — reachable only once bonded attestation exists
        TRANCHE_1_RELEASED,
        FINAL_WINDOW,
        FULLY_RELEASED,
        CLOSED
    }

    // ─── Events ──────────────────────────────────────────────────────────
    event VaultCreated(uint256 indexed vaultId, address indexed owner, bytes32 planCommitmentHash);
    event CheckIn(uint256 indexed vaultId, uint256 nextDeadline);
    event VaultFunded(uint256 indexed vaultId, uint256 amount, uint256 totalBalance);
    event StateTransition(uint256 indexed vaultId, VaultState from, VaultState to);
    event LegalDocAnchored(uint256 indexed vaultId, bytes32 legalDocHash);
    event AttestationRequested(uint256 indexed vaultId);
    event QuorumResultSubmitted(uint256 indexed vaultId, bool quorumMet);
    event GuardianHalt(uint256 indexed vaultId, address guardian);
    event TrancheReleased(uint256 indexed vaultId, uint256 tranche, uint256 amount);
    event VaultFullyReleased(uint256 indexed vaultId);
    event VaultCancelled(uint256 indexed vaultId);

    // ─── Core Entry Points ──────────────────────────────────────────────

    function createVault(
        bytes32 planCommitmentHash,
        address fundingAsset,
        uint256 checkInIntervalSeconds,
        uint256 graceWindowSeconds,
        uint256 disputeWindowSeconds,
        uint256 finalWindowSeconds,
        address guardianHaltKey
    ) external returns (uint256 vaultId);

    function checkIn(uint256 vaultId, bytes calldata signature) external;

    function fundVault(uint256 vaultId, uint256 amount) external;

    function anchorLegalDoc(uint256 vaultId, bytes32 legalDocHash) external;

    function requestAttestation(uint256 vaultId) external;

    function submitQuorumResult(
        uint256 vaultId,
        bool quorumMet,
        bytes calldata fceSignature
    ) external;

    function guardianHalt(uint256 vaultId) external;

    function finalizeDisputeWindow(uint256 vaultId) external;

    function finalizeFinalWindow(uint256 vaultId) external;

    function cancelVault(uint256 vaultId) external;
}
