// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {VaultRegistry} from "../src/VaultRegistry.sol";
import {IVaultRegistry} from "../src/interfaces/IVaultRegistry.sol";
import {MockERC20} from "./mocks/MockERC20.sol";

/**
 * @title VaultRegistryTest
 * @notice Layer 1 test suite — walks a vault through every state transition
 *         in the architecture's lifecycle diagram, against mocked dependencies.
 *
 *         Tests cover:
 *         1. Happy path: ACTIVE → WARNING → QUORUM_PENDING → DISPUTE_WINDOW
 *            → TRANCHE_1_RELEASED → FINAL_WINDOW → FULLY_RELEASED
 *         2. Check-in recovery: WARNING → ACTIVE
 *         3. Owner override: QUORUM_PENDING → ACTIVE
 *         4. Guardian halt from DISPUTE_WINDOW → ACTIVE
 *         5. Guardian halt from FINAL_WINDOW → ACTIVE
 *         6. Cancel vault: ACTIVE → CLOSED
 *         7. Access control on every restricted function
 *         8. Window timing edge cases
 */
contract VaultRegistryTest is Test {
    VaultRegistry public registry;
    MockERC20 public mockFXRP;

    // Actors
    address public owner = makeAddr("owner");
    address public guardian = makeAddr("guardian");
    address public enclaveOracle = makeAddr("enclaveOracle");
    address public relayer = makeAddr("relayer");
    address public attacker = makeAddr("attacker");

    // Demo-length timing (seconds) — all short for testing
    uint256 constant CHECK_IN_INTERVAL = 300;  // 5 min
    uint256 constant GRACE_WINDOW = 120;       // 2 min
    uint256 constant DISPUTE_WINDOW = 180;     // 3 min
    uint256 constant FINAL_WINDOW = 120;       // 2 min

    // Vault creation params
    bytes32 constant PLAN_HASH = keccak256("sealed-plan-v1");
    uint256 constant FUND_AMOUNT = 1000 ether;

    // ─── Setup ───────────────────────────────────────────────────────────

    function setUp() public {
        registry = new VaultRegistry(enclaveOracle);
        mockFXRP = new MockERC20("Fake FXRP", "FXRP", 18);

        // Fund the owner with mock tokens
        mockFXRP.mint(owner, 10_000 ether);

        // Owner approves VaultRegistry to pull tokens
        vm.prank(owner);
        mockFXRP.approve(address(registry), type(uint256).max);
    }

    // ─── Helpers ─────────────────────────────────────────────────────────

    function _createDefaultVault() internal returns (uint256 vaultId) {
        vm.prank(owner);
        vaultId = registry.createVault(
            PLAN_HASH,
            address(mockFXRP),
            CHECK_IN_INTERVAL,
            GRACE_WINDOW,
            DISPUTE_WINDOW,
            FINAL_WINDOW,
            guardian
        );
    }

    function _createAndFundVault() internal returns (uint256 vaultId) {
        vaultId = _createDefaultVault();
        vm.prank(owner);
        registry.fundVault(vaultId, FUND_AMOUNT);
    }

    function _advanceToWarning(uint256 vaultId) internal {
        // Advance past check-in deadline
        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        // Anyone can mark warning
        vm.prank(relayer);
        registry.markWarning(vaultId);
    }

    function _advanceToQuorumPending(uint256 vaultId) internal {
        _advanceToWarning(vaultId);
        // Advance past grace window
        vm.warp(block.timestamp + GRACE_WINDOW + 1);
        // Request attestation (transitions WARNING → QUORUM_PENDING)
        vm.prank(relayer);
        registry.requestAttestation(vaultId);
    }

    function _advanceToDisputeWindow(uint256 vaultId) internal {
        _advanceToQuorumPending(vaultId);
        // Enclave oracle submits quorum met
        vm.prank(enclaveOracle);
        registry.submitQuorumResult(vaultId, true, "");
    }

    function _advanceToFinalWindow(uint256 vaultId) internal {
        _advanceToDisputeWindow(vaultId);
        // Advance past dispute window
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);
    }

    function _getVaultState(uint256 vaultId) internal view returns (IVaultRegistry.VaultState) {
        return registry.getVaultState(vaultId);
    }

    function _getVaultBalance(uint256 vaultId) internal view returns (uint256) {
        return registry.getVaultBalance(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 1: Vault Creation
    // ═══════════════════════════════════════════════════════════════════════

    function test_CreateVault() public {
        vm.prank(owner);
        uint256 vaultId = registry.createVault(
            PLAN_HASH,
            address(mockFXRP),
            CHECK_IN_INTERVAL,
            GRACE_WINDOW,
            DISPUTE_WINDOW,
            FINAL_WINDOW,
            guardian
        );

        assertEq(vaultId, 1, "First vault should have ID 1");
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));

        (address vaultOwner, bytes32 commitment, address asset,,) = registry.getVaultConfig(vaultId);
        assertEq(vaultOwner, owner);
        assertEq(commitment, PLAN_HASH);
        assertEq(asset, address(mockFXRP));
    }

    function test_CreateVault_IncrementingIds() public {
        uint256 id1 = _createDefaultVault();
        uint256 id2 = _createDefaultVault();
        assertEq(id1, 1);
        assertEq(id2, 2);
    }

    function test_CreateVault_RevertOnZeroCommitment() public {
        vm.prank(owner);
        vm.expectRevert("VR: empty commitment");
        registry.createVault(bytes32(0), address(mockFXRP), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, guardian);
    }

    function test_CreateVault_RevertOnZeroAsset() public {
        vm.prank(owner);
        vm.expectRevert("VR: zero asset");
        registry.createVault(PLAN_HASH, address(0), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, guardian);
    }

    function test_CreateVault_RevertOnZeroGuardian() public {
        vm.prank(owner);
        vm.expectRevert("VR: zero guardian");
        registry.createVault(PLAN_HASH, address(mockFXRP), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, address(0));
    }

    function test_CreateVault_EmitsEvents() public {
        vm.prank(owner);
        vm.expectEmit(true, true, false, true);
        emit IVaultRegistry.VaultCreated(1, owner, PLAN_HASH);
        registry.createVault(PLAN_HASH, address(mockFXRP), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, guardian);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 2: Funding
    // ═══════════════════════════════════════════════════════════════════════

    function test_FundVault() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(owner);
        registry.fundVault(vaultId, FUND_AMOUNT);

        assertEq(_getVaultBalance(vaultId), FUND_AMOUNT);
        assertEq(mockFXRP.balanceOf(address(registry)), FUND_AMOUNT);
    }

    function test_FundVault_MultipleFundings() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(owner);
        registry.fundVault(vaultId, 500 ether);
        vm.prank(owner);
        registry.fundVault(vaultId, 500 ether);

        assertEq(_getVaultBalance(vaultId), 1000 ether);
    }

    function test_FundVault_RevertNotOwner() public {
        uint256 vaultId = _createDefaultVault();
        vm.prank(attacker);
        vm.expectRevert("VR: not owner");
        registry.fundVault(vaultId, FUND_AMOUNT);
    }

    function test_FundVault_RevertZeroAmount() public {
        uint256 vaultId = _createDefaultVault();
        vm.prank(owner);
        vm.expectRevert("VR: zero amount");
        registry.fundVault(vaultId, 0);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 3: Check-in & WARNING recovery
    // ═══════════════════════════════════════════════════════════════════════

    function test_CheckIn_ResetsDeadline() public {
        uint256 vaultId = _createDefaultVault();

        // Advance time but still within interval
        vm.warp(block.timestamp + 100);

        vm.prank(owner);
        registry.checkIn(vaultId, "");

        (uint256 lastCheckIn, uint256 windowDeadline,,,,) = registry.getVaultTiming(vaultId);
        assertEq(lastCheckIn, block.timestamp);
        assertEq(windowDeadline, block.timestamp + CHECK_IN_INTERVAL);
    }

    function test_CheckIn_RecoverFromWarning() public {
        uint256 vaultId = _createDefaultVault();
        _advanceToWarning(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.WARNING));

        // Owner checks in during grace period → back to ACTIVE
        vm.prank(owner);
        registry.checkIn(vaultId, "");

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
    }

    function test_CheckIn_RevertNotOwner() public {
        uint256 vaultId = _createDefaultVault();
        vm.prank(attacker);
        vm.expectRevert("VR: not owner");
        registry.checkIn(vaultId, "");
    }

    function test_CheckIn_RevertFromDisputeWindow() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToDisputeWindow(vaultId);

        vm.prank(owner);
        vm.expectRevert("VR: cannot check in from this state");
        registry.checkIn(vaultId, "");
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 4: ACTIVE → WARNING → QUORUM_PENDING
    // ═══════════════════════════════════════════════════════════════════════

    function test_MarkWarning_OnMissedCheckIn() public {
        uint256 vaultId = _createDefaultVault();

        // Still within interval — should revert
        vm.prank(relayer);
        vm.expectRevert("VR: check-in not missed");
        registry.markWarning(vaultId);

        // Advance past deadline
        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);

        vm.prank(relayer);
        registry.markWarning(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.WARNING));
    }

    function test_IsCheckInMissed() public {
        uint256 vaultId = _createDefaultVault();

        assertFalse(registry.isCheckInMissed(vaultId));

        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        assertTrue(registry.isCheckInMissed(vaultId));
    }

    function test_RequestAttestation_GraceExpired() public {
        uint256 vaultId = _createDefaultVault();
        _advanceToWarning(vaultId);

        // Grace not expired yet — should revert
        vm.prank(relayer);
        vm.expectRevert("VR: not in WARNING or grace not expired");
        registry.requestAttestation(vaultId);

        // Advance past grace window
        vm.warp(block.timestamp + GRACE_WINDOW + 1);

        vm.prank(relayer);
        registry.requestAttestation(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.QUORUM_PENDING));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 5: Owner Override from QUORUM_PENDING
    // ═══════════════════════════════════════════════════════════════════════

    function test_OwnerOverride_FromQuorumPending() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToQuorumPending(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.QUORUM_PENDING));

        vm.prank(owner);
        registry.ownerOverride(vaultId, "");

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
    }

    function test_OwnerOverride_RevertNotOwner() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToQuorumPending(vaultId);

        vm.prank(attacker);
        vm.expectRevert("VR: not owner");
        registry.ownerOverride(vaultId, "");
    }

    function test_OwnerOverride_RevertWrongState() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(owner);
        vm.expectRevert("VR: wrong state");
        registry.ownerOverride(vaultId, "");
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 6: Quorum Result → DISPUTE_WINDOW
    // ═══════════════════════════════════════════════════════════════════════

    function test_SubmitQuorumResult_Met() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToQuorumPending(vaultId);

        vm.prank(enclaveOracle);
        registry.submitQuorumResult(vaultId, true, "");

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));
    }

    function test_SubmitQuorumResult_NotMet_StaysInQuorumPending() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToQuorumPending(vaultId);

        vm.prank(enclaveOracle);
        registry.submitQuorumResult(vaultId, false, "");

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.QUORUM_PENDING));
    }

    function test_SubmitQuorumResult_RevertNotOracle() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToQuorumPending(vaultId);

        vm.prank(attacker);
        vm.expectRevert("VR: not enclave oracle");
        registry.submitQuorumResult(vaultId, true, "");
    }

    function test_SubmitQuorumResult_RevertWrongState() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(enclaveOracle);
        vm.expectRevert("VR: wrong state");
        registry.submitQuorumResult(vaultId, true, "");
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 7: Guardian Halt — Design Principle #4
    // ═══════════════════════════════════════════════════════════════════════

    function test_GuardianHalt_FromDisputeWindow() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToDisputeWindow(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));

        vm.prank(guardian);
        registry.guardianHalt(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
    }

    function test_GuardianHalt_FromFinalWindow() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToFinalWindow(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FINAL_WINDOW));

        vm.prank(guardian);
        registry.guardianHalt(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
    }

    function test_GuardianHalt_RevertNotGuardian() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToDisputeWindow(vaultId);

        vm.prank(attacker);
        vm.expectRevert("VR: not guardian");
        registry.guardianHalt(vaultId);
    }

    function test_GuardianHalt_RevertFromActive() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(guardian);
        vm.expectRevert("VR: cannot halt from this state");
        registry.guardianHalt(vaultId);
    }

    function test_GuardianHalt_RevertFromQuorumPending() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToQuorumPending(vaultId);

        vm.prank(guardian);
        vm.expectRevert("VR: cannot halt from this state");
        registry.guardianHalt(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 8: Happy Path — Full Lifecycle with Two-Step Release
    // ═══════════════════════════════════════════════════════════════════════

    function test_FullLifecycle_HappyPath() public {
        // --- Create & Fund ---
        uint256 vaultId = _createAndFundVault();
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
        assertEq(_getVaultBalance(vaultId), FUND_AMOUNT);

        // --- Miss check-in → WARNING ---
        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        vm.prank(relayer);
        registry.markWarning(vaultId);
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.WARNING));

        // --- Grace expires → QUORUM_PENDING ---
        vm.warp(block.timestamp + GRACE_WINDOW + 1);
        vm.prank(relayer);
        registry.requestAttestation(vaultId);
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.QUORUM_PENDING));

        // --- Quorum met → DISPUTE_WINDOW ---
        vm.prank(enclaveOracle);
        registry.submitQuorumResult(vaultId, true, "");
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));

        // --- Dispute window elapses → TRANCHE_1 → FINAL_WINDOW ---
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);
        // After finalizeDisputeWindow, state is FINAL_WINDOW (TRANCHE_1_RELEASED is transient)
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FINAL_WINDOW));

        // Balance should be halved after tranche 1
        assertEq(_getVaultBalance(vaultId), FUND_AMOUNT / 2);

        // --- Final window elapses → FULLY_RELEASED ---
        vm.warp(block.timestamp + FINAL_WINDOW + 1);
        registry.finalizeFinalWindow(vaultId);
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FULLY_RELEASED));

        // All funds released
        assertEq(_getVaultBalance(vaultId), 0);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 9: Dispute/Final Window Timing
    // ═══════════════════════════════════════════════════════════════════════

    function test_FinalizeDisputeWindow_RevertBeforeExpiry() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToDisputeWindow(vaultId);

        // Try to finalize before window elapses
        vm.expectRevert("VR: dispute window not elapsed");
        registry.finalizeDisputeWindow(vaultId);
    }

    function test_FinalizeFinalWindow_RevertBeforeExpiry() public {
        uint256 vaultId = _createAndFundVault();
        _advanceToFinalWindow(vaultId);

        // Try to finalize before window elapses
        vm.expectRevert("VR: final window not elapsed");
        registry.finalizeFinalWindow(vaultId);
    }

    function test_FinalizeDisputeWindow_RevertWrongState() public {
        uint256 vaultId = _createDefaultVault();

        vm.expectRevert("VR: wrong state");
        registry.finalizeDisputeWindow(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 10: Cancel Vault
    // ═══════════════════════════════════════════════════════════════════════

    function test_CancelVault_RefundsFunds() public {
        uint256 vaultId = _createAndFundVault();

        uint256 ownerBalanceBefore = mockFXRP.balanceOf(owner);

        vm.prank(owner);
        registry.cancelVault(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.CLOSED));
        assertEq(_getVaultBalance(vaultId), 0);
        assertEq(mockFXRP.balanceOf(owner), ownerBalanceBefore + FUND_AMOUNT);
    }

    function test_CancelVault_NoFunds() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(owner);
        registry.cancelVault(vaultId);

        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.CLOSED));
    }

    function test_CancelVault_RevertNotOwner() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(attacker);
        vm.expectRevert("VR: not owner");
        registry.cancelVault(vaultId);
    }

    function test_CancelVault_RevertFromWarning() public {
        uint256 vaultId = _createDefaultVault();
        _advanceToWarning(vaultId);

        vm.prank(owner);
        vm.expectRevert("VR: wrong state");
        registry.cancelVault(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 11: Legal Anchor
    // ═══════════════════════════════════════════════════════════════════════

    function test_AnchorLegalDoc() public {
        uint256 vaultId = _createDefaultVault();
        bytes32 docHash = keccak256("my-will-hash");

        vm.prank(owner);
        registry.anchorLegalDoc(vaultId, docHash);

        (,,,, bytes32 storedHash) = registry.getVaultConfig(vaultId);
        assertEq(storedHash, docHash);
    }

    function test_AnchorLegalDoc_RevertNotOwner() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(attacker);
        vm.expectRevert("VR: not owner");
        registry.anchorLegalDoc(vaultId, keccak256("doc"));
    }

    function test_AnchorLegalDoc_RevertEmptyHash() public {
        uint256 vaultId = _createDefaultVault();

        vm.prank(owner);
        vm.expectRevert("VR: empty hash");
        registry.anchorLegalDoc(vaultId, bytes32(0));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 12: Full lifecycle with guardian halt mid-trigger
    // ═══════════════════════════════════════════════════════════════════════

    function test_FullLifecycle_GuardianHaltDuringDispute() public {
        uint256 vaultId = _createAndFundVault();

        // Progress to DISPUTE_WINDOW
        _advanceToDisputeWindow(vaultId);
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));

        // Guardian halts — Design Principle #4
        vm.prank(guardian);
        registry.guardianHalt(vaultId);
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));

        // No funds moved
        assertEq(_getVaultBalance(vaultId), FUND_AMOUNT);

        // Owner can now check in normally
        vm.prank(owner);
        registry.checkIn(vaultId, "");
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
    }

    function test_FullLifecycle_GuardianHaltDuringFinalWindow() public {
        uint256 vaultId = _createAndFundVault();

        // Progress to FINAL_WINDOW (tranche 1 already released)
        _advanceToFinalWindow(vaultId);
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FINAL_WINDOW));
        assertEq(_getVaultBalance(vaultId), FUND_AMOUNT / 2); // tranche 1 deducted

        // Guardian halts even after tranche 1 — saves the remaining funds
        vm.prank(guardian);
        registry.guardianHalt(vaultId);
        assertEq(uint256(_getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));

        // Remaining balance preserved
        assertEq(_getVaultBalance(vaultId), FUND_AMOUNT / 2);
    }
}
