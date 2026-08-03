// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IERC20} from "forge-std/interfaces/IERC20.sol";
import {IVaultRegistry} from "./interfaces/IVaultRegistry.sol";

/**
 * @title VaultRegistry
 * @author Continuity Vault Team
 * @notice The canonical on-chain state machine for Continuity Vault.
 *
 * @dev Holds plan commitment hashes only — never the plan itself.
 *      All timing windows are configurable per-vault for demo vs production use.
 *
 *      Lifecycle (from continuity-vault-architecture.md):
 *        ACTIVE → WARNING → QUORUM_PENDING → DISPUTE_WINDOW
 *        → TRANCHE_1_RELEASED → FINAL_WINDOW → FULLY_RELEASED
 *
 *      Design principles enforced:
 *        #1 Trust-minimize the trigger — quorum, not a single signal
 *        #2 Optimistic execution — dispute window before any release
 *        #4 Owner can self-correct mid-trigger — guardian halt in DISPUTE_WINDOW + FINAL_WINDOW
 *
 *      Layer 0 scaffold: structure and storage laid out; full logic is Layer 1.
 */
contract VaultRegistry is IVaultRegistry {
    // ─── Storage ─────────────────────────────────────────────────────────

    struct Vault {
        // Identity
        address owner;
        bytes32 planCommitmentHash;
        address fundingAsset;        // FXRP ERC-20 address
        uint256 balance;
        // Timing config (seconds) — set at creation, demo uses minutes
        uint256 checkInInterval;
        uint256 graceWindow;
        uint256 disputeWindow;
        uint256 finalWindow;
        // Current state
        VaultState state;
        uint256 lastCheckIn;         // timestamp of last check-in
        uint256 windowDeadline;      // timestamp when current window expires
        // Access control
        address guardianHaltKey;
        // Legal anchor (cheap Phase-2 prep, built in Layer 1)
        bytes32 legalDocHash;
    }

    /// @notice Auto-incrementing vault ID counter.
    uint256 public nextVaultId = 1;

    /// @notice Vault ID -> Vault data.
    mapping(uint256 => Vault) public vaults;

    /// @notice The address authorized to submit quorum results (enclave oracle).
    address public enclaveOracle;

    /// @notice The FDC attestation verifier contract (Layer 2).
    address public fdcVerifier;

    /// @notice Vault ID -> number of verified FDC attestations received.
    mapping(uint256 => uint256) public vaultAttestationCount;

    /// @notice Default quorum threshold (MVP: 2 attestations).
    uint256 public quorumThreshold = 2;

    // ─── Modifiers ───────────────────────────────────────────────────────

    modifier onlyOwner(uint256 vaultId) {
        require(msg.sender == vaults[vaultId].owner, "VR: not owner");
        _;
    }

    modifier onlyGuardian(uint256 vaultId) {
        require(msg.sender == vaults[vaultId].guardianHaltKey, "VR: not guardian");
        _;
    }

    modifier onlyEnclaveOracle() {
        require(msg.sender == enclaveOracle, "VR: not enclave oracle");
        _;
    }

    modifier onlyFdcVerifier() {
        require(msg.sender == fdcVerifier, "VR: not fdc verifier");
        _;
    }

    modifier inState(uint256 vaultId, VaultState expected) {
        require(vaults[vaultId].state == expected, "VR: wrong state");
        _;
    }

    // ─── Constructor ─────────────────────────────────────────────────────

    /// @param _enclaveOracle Address authorized to submit quorum results.
    constructor(address _enclaveOracle) {
        require(_enclaveOracle != address(0), "VR: zero oracle");
        enclaveOracle = _enclaveOracle;
    }

    /// @notice Set the FDC attestation verifier address (Layer 2).
    /// @dev Only callable once (no admin key in MVP).
    function setFdcVerifier(address _fdcVerifier) external {
        require(fdcVerifier == address(0), "VR: fdc verifier already set");
        require(_fdcVerifier != address(0), "VR: zero fdc verifier");
        fdcVerifier = _fdcVerifier;
    }

    /// @notice Set quorum threshold.
    function setQuorumThreshold(uint256 _threshold) external {
        require(_threshold > 0, "VR: zero threshold");
        quorumThreshold = _threshold;
    }

    /// @notice Called by FdcAttestationVerifier when a verified attestation is recorded.
    /// @dev Increments the attestation count. If quorum is met and vault is in
    ///      QUORUM_PENDING, auto-transitions to DISPUTE_WINDOW.
    function submitVerifiedAttestation(uint256 vaultId)
        external
        onlyFdcVerifier()
    {
        vaultAttestationCount[vaultId]++;

        // Auto-transition if quorum met and vault is waiting for quorum
        Vault storage v = vaults[vaultId];
        if (v.state == VaultState.QUORUM_PENDING && vaultAttestationCount[vaultId] >= quorumThreshold) {
            v.state = VaultState.DISPUTE_WINDOW;
            v.windowDeadline = block.timestamp + v.disputeWindow;
            emit StateTransition(vaultId, VaultState.QUORUM_PENDING, VaultState.DISPUTE_WINDOW);
            emit QuorumResultSubmitted(vaultId, true);
        }
    }

    // ─── Core Entry Points ──────────────────────────────────────────────
    // Full implementation is Layer 1. Scaffolded here for compilation.

    /// @inheritdoc IVaultRegistry
    function createVault(
        bytes32 planCommitmentHash,
        address fundingAsset,
        uint256 checkInIntervalSeconds,
        uint256 graceWindowSeconds,
        uint256 disputeWindowSeconds,
        uint256 finalWindowSeconds,
        address guardianHaltKey
    ) external returns (uint256 vaultId) {
        require(planCommitmentHash != bytes32(0), "VR: empty commitment");
        require(fundingAsset != address(0), "VR: zero asset");
        require(checkInIntervalSeconds > 0, "VR: zero check-in interval");
        require(graceWindowSeconds > 0, "VR: zero grace window");
        require(disputeWindowSeconds > 0, "VR: zero dispute window");
        require(finalWindowSeconds > 0, "VR: zero final window");
        require(guardianHaltKey != address(0), "VR: zero guardian");

        vaultId = nextVaultId++;

        Vault storage v = vaults[vaultId];
        v.owner = msg.sender;
        v.planCommitmentHash = planCommitmentHash;
        v.fundingAsset = fundingAsset;
        v.checkInInterval = checkInIntervalSeconds;
        v.graceWindow = graceWindowSeconds;
        v.disputeWindow = disputeWindowSeconds;
        v.finalWindow = finalWindowSeconds;
        v.guardianHaltKey = guardianHaltKey;
        v.state = VaultState.ACTIVE;
        v.lastCheckIn = block.timestamp;
        v.windowDeadline = block.timestamp + checkInIntervalSeconds;

        emit VaultCreated(vaultId, msg.sender, planCommitmentHash);
        emit StateTransition(vaultId, VaultState.CLOSED, VaultState.ACTIVE); // CLOSED used as "none"
    }

    /// @inheritdoc IVaultRegistry
    function checkIn(uint256 vaultId, bytes calldata /* signature */)
        external
        onlyOwner(vaultId)
    {
        Vault storage v = vaults[vaultId];
        require(
            v.state == VaultState.ACTIVE || v.state == VaultState.WARNING,
            "VR: cannot check in from this state"
        );

        VaultState prev = v.state;
        v.state = VaultState.ACTIVE;
        v.lastCheckIn = block.timestamp;
        v.windowDeadline = block.timestamp + v.checkInInterval;

        if (prev != VaultState.ACTIVE) {
            emit StateTransition(vaultId, prev, VaultState.ACTIVE);
        }
        emit CheckIn(vaultId, v.windowDeadline);
    }

    /// @inheritdoc IVaultRegistry
    function fundVault(uint256 vaultId, uint256 amount)
        external
        onlyOwner(vaultId)
    {
        require(amount > 0, "VR: zero amount");
        Vault storage v = vaults[vaultId];
        require(v.state != VaultState.FULLY_RELEASED && v.state != VaultState.CLOSED, "VR: vault closed");

        IERC20(v.fundingAsset).transferFrom(msg.sender, address(this), amount);
        v.balance += amount;

        emit VaultFunded(vaultId, amount, v.balance);
    }

    /// @inheritdoc IVaultRegistry
    function anchorLegalDoc(uint256 vaultId, bytes32 legalDocHash)
        external
        onlyOwner(vaultId)
    {
        require(legalDocHash != bytes32(0), "VR: empty hash");
        vaults[vaultId].legalDocHash = legalDocHash;
        emit LegalDocAnchored(vaultId, legalDocHash);
    }

    /// @inheritdoc IVaultRegistry
    function requestAttestation(uint256 vaultId) external {
        Vault storage v = vaults[vaultId];
        // Transition WARNING → QUORUM_PENDING when grace period expires
        if (v.state == VaultState.WARNING && block.timestamp >= v.windowDeadline) {
            v.state = VaultState.QUORUM_PENDING;
            emit StateTransition(vaultId, VaultState.WARNING, VaultState.QUORUM_PENDING);
            emit AttestationRequested(vaultId);
        } else {
            revert("VR: not in WARNING or grace not expired");
        }
    }

    /// @inheritdoc IVaultRegistry
    function submitQuorumResult(
        uint256 vaultId,
        bool quorumMet,
        bytes calldata /* fceSignature */
    )
        external
        onlyEnclaveOracle()
        inState(vaultId, VaultState.QUORUM_PENDING)
    {
        Vault storage v = vaults[vaultId];

        if (quorumMet) {
            v.state = VaultState.DISPUTE_WINDOW;
            v.windowDeadline = block.timestamp + v.disputeWindow;
            emit StateTransition(vaultId, VaultState.QUORUM_PENDING, VaultState.DISPUTE_WINDOW);
        }
        // If quorum not met, stay in QUORUM_PENDING (more attestations can arrive)

        emit QuorumResultSubmitted(vaultId, quorumMet);
    }

    /// @inheritdoc IVaultRegistry
    function guardianHalt(uint256 vaultId)
        external
        onlyGuardian(vaultId)
    {
        Vault storage v = vaults[vaultId];
        require(
            v.state == VaultState.DISPUTE_WINDOW || v.state == VaultState.FINAL_WINDOW,
            "VR: cannot halt from this state"
        );

        VaultState prev = v.state;
        v.state = VaultState.ACTIVE;
        v.lastCheckIn = block.timestamp;
        v.windowDeadline = block.timestamp + v.checkInInterval;

        emit GuardianHalt(vaultId, msg.sender);
        emit StateTransition(vaultId, prev, VaultState.ACTIVE);
    }

    /// @inheritdoc IVaultRegistry
    function finalizeDisputeWindow(uint256 vaultId)
        external
        inState(vaultId, VaultState.DISPUTE_WINDOW)
    {
        Vault storage v = vaults[vaultId];
        require(block.timestamp >= v.windowDeadline, "VR: dispute window not elapsed");

        // Release tranche 1 (50% for MVP two-step release)
        uint256 tranche1Amount = v.balance / 2;
        v.balance -= tranche1Amount;

        v.state = VaultState.TRANCHE_1_RELEASED;
        emit StateTransition(vaultId, VaultState.DISPUTE_WINDOW, VaultState.TRANCHE_1_RELEASED);
        emit TrancheReleased(vaultId, 1, tranche1Amount);

        // Immediately transition to FINAL_WINDOW
        v.state = VaultState.FINAL_WINDOW;
        v.windowDeadline = block.timestamp + v.finalWindow;
        emit StateTransition(vaultId, VaultState.TRANCHE_1_RELEASED, VaultState.FINAL_WINDOW);
    }

    /// @inheritdoc IVaultRegistry
    function finalizeFinalWindow(uint256 vaultId)
        external
        inState(vaultId, VaultState.FINAL_WINDOW)
    {
        Vault storage v = vaults[vaultId];
        require(block.timestamp >= v.windowDeadline, "VR: final window not elapsed");

        uint256 remainingBalance = v.balance;
        v.balance = 0;

        v.state = VaultState.FULLY_RELEASED;
        emit StateTransition(vaultId, VaultState.FINAL_WINDOW, VaultState.FULLY_RELEASED);
        emit TrancheReleased(vaultId, 2, remainingBalance);
        emit VaultFullyReleased(vaultId);
    }

    /// @inheritdoc IVaultRegistry
    function cancelVault(uint256 vaultId)
        external
        onlyOwner(vaultId)
        inState(vaultId, VaultState.ACTIVE)
    {
        Vault storage v = vaults[vaultId];

        // Return funds to owner
        uint256 refund = v.balance;
        v.balance = 0;

        if (refund > 0) {
            IERC20(v.fundingAsset).transfer(msg.sender, refund);
        }

        v.state = VaultState.CLOSED;
        emit StateTransition(vaultId, VaultState.ACTIVE, VaultState.CLOSED);
        emit VaultCancelled(vaultId);
    }

    // ─── View Helpers ────────────────────────────────────────────────────

    /// @notice Check if a vault's check-in deadline has been missed (for relayer).
    function isCheckInMissed(uint256 vaultId) external view returns (bool) {
        Vault storage v = vaults[vaultId];
        return v.state == VaultState.ACTIVE && block.timestamp > v.windowDeadline;
    }

    /// @notice Transition ACTIVE → WARNING when check-in is missed.
    /// @dev Callable by anyone (relayer). Enables permissionless state progression.
    function markWarning(uint256 vaultId)
        external
        inState(vaultId, VaultState.ACTIVE)
    {
        Vault storage v = vaults[vaultId];
        require(block.timestamp > v.windowDeadline, "VR: check-in not missed");

        v.state = VaultState.WARNING;
        v.windowDeadline = block.timestamp + v.graceWindow;

        emit StateTransition(vaultId, VaultState.ACTIVE, VaultState.WARNING);
    }

    /// @notice Transition QUORUM_PENDING → ACTIVE via owner override (before quorum).
    function ownerOverride(uint256 vaultId, bytes calldata /* signature */)
        external
        onlyOwner(vaultId)
        inState(vaultId, VaultState.QUORUM_PENDING)
    {
        Vault storage v = vaults[vaultId];
        v.state = VaultState.ACTIVE;
        v.lastCheckIn = block.timestamp;
        v.windowDeadline = block.timestamp + v.checkInInterval;

        emit StateTransition(vaultId, VaultState.QUORUM_PENDING, VaultState.ACTIVE);
    }

    /// @notice Get vault state.
    function getVaultState(uint256 vaultId) external view returns (VaultState) {
        return vaults[vaultId].state;
    }

    /// @notice Get vault balance.
    function getVaultBalance(uint256 vaultId) external view returns (uint256) {
        return vaults[vaultId].balance;
    }

    /// @notice Get vault owner.
    function getVaultOwner(uint256 vaultId) external view returns (address) {
        return vaults[vaultId].owner;
    }

    /// @notice Get vault identity fields.
    function getVaultConfig(uint256 vaultId) external view returns (
        address owner,
        bytes32 planCommitmentHash,
        address fundingAsset,
        address guardianHaltKey,
        bytes32 legalDocHash
    ) {
        Vault storage v = vaults[vaultId];
        return (v.owner, v.planCommitmentHash, v.fundingAsset, v.guardianHaltKey, v.legalDocHash);
    }

    /// @notice Get vault timing fields.
    function getVaultTiming(uint256 vaultId) external view returns (
        uint256 lastCheckIn,
        uint256 windowDeadline,
        uint256 checkInInterval,
        uint256 graceWindow,
        uint256 disputeWindow,
        uint256 finalWindow
    ) {
        Vault storage v = vaults[vaultId];
        return (v.lastCheckIn, v.windowDeadline, v.checkInInterval, v.graceWindow, v.disputeWindow, v.finalWindow);
    }
}
