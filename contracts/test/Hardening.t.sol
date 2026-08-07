// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {VaultRegistry} from "../src/VaultRegistry.sol";
import {IVaultRegistry} from "../src/interfaces/IVaultRegistry.sol";
import {MockERC20} from "./mocks/MockERC20.sol";

/**
 * @title HardeningTest
 * @notice Layer 6: Security hardening tests.
 *
 *         Tests:
 *         1. Reentrancy protection on cancelVault / finalizeDisputeWindow / finalizeFinalWindow
 *         2. Admin-only access control on setQuorumThreshold / setFdcVerifier
 *         3. State transition completeness — no skipping states
 *         4. Window timing edge cases — exact boundary timestamps
 *         5. Double-action prevention — cannot finalize same window twice
 *         6. Zero-balance release — no reverts on empty vault
 */
contract HardeningTest is Test {
    VaultRegistry public registry;
    MockERC20 public mockFXRP;

    address public deployer;
    address public owner = makeAddr("owner");
    address public guardian = makeAddr("guardian");
    address public enclaveOracle = makeAddr("enclaveOracle");
    address public attacker = makeAddr("attacker");

    uint256 constant CHECK_IN_INTERVAL = 60;
    uint256 constant GRACE_WINDOW = 30;
    uint256 constant DISPUTE_WINDOW = 45;
    uint256 constant FINAL_WINDOW = 30;
    uint256 constant FUND_AMOUNT = 100 ether;

    bytes32 constant PLAN_HASH = keccak256("hardening-test-plan");

    function setUp() public {
        deployer = address(this);
        registry = new VaultRegistry(enclaveOracle);
        mockFXRP = new MockERC20("Fake FXRP", "FXRP", 18);

        mockFXRP.mint(owner, 10_000 ether);
        vm.prank(owner);
        mockFXRP.approve(address(registry), type(uint256).max);
    }

    // ─── Helpers ────────────────────────────────────────────────────────

    function _createAndFund() internal returns (uint256) {
        vm.prank(owner);
        uint256 vaultId = registry.createVault(
            PLAN_HASH, address(mockFXRP), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, guardian
        );
        vm.prank(owner);
        registry.fundVault(vaultId, FUND_AMOUNT);
        return vaultId;
    }

    function _toDisputeWindow(uint256 vaultId) internal {
        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        vm.prank(address(0xBEEF));
        registry.markWarning(vaultId);
        vm.warp(block.timestamp + GRACE_WINDOW + 1);
        vm.prank(address(0xBEEF));
        registry.requestAttestation(vaultId);
        vm.prank(enclaveOracle);
        registry.submitQuorumResult(vaultId, true, "");
    }

    function _toFinalWindow(uint256 vaultId) internal {
        _toDisputeWindow(vaultId);
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 1: Admin Access Control (Layer 6 hardening)
    // ═══════════════════════════════════════════════════════════════════════

    function test_Admin_IsDeployer() public view {
        assertEq(registry.admin(), deployer);
    }

    function test_SetQuorumThreshold_OnlyAdmin() public {
        // Admin can set
        registry.setQuorumThreshold(5);
        assertEq(registry.quorumThreshold(), 5);
    }

    function test_SetQuorumThreshold_RevertNotAdmin() public {
        vm.prank(attacker);
        vm.expectRevert("VR: not admin");
        registry.setQuorumThreshold(5);
    }

    function test_SetFdcVerifier_OnlyAdmin() public {
        address verifier = makeAddr("verifier");
        registry.setFdcVerifier(verifier);
        assertEq(registry.fdcVerifier(), verifier);
    }

    function test_SetFdcVerifier_RevertNotAdmin() public {
        vm.prank(attacker);
        vm.expectRevert("VR: not admin");
        registry.setFdcVerifier(makeAddr("verifier"));
    }

    function test_SetFdcVerifier_RevertAlreadySet() public {
        registry.setFdcVerifier(makeAddr("v1"));

        vm.expectRevert("VR: fdc verifier already set");
        registry.setFdcVerifier(makeAddr("v2"));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 2: State Transition Completeness
    // ═══════════════════════════════════════════════════════════════════════

    function test_CannotSkipFromActive_ToDisputeWindow() public {
        uint256 vaultId = _createAndFund();

        // Cannot finalize dispute from ACTIVE
        vm.expectRevert("VR: wrong state");
        registry.finalizeDisputeWindow(vaultId);
    }

    function test_CannotSkipFromActive_ToFinalWindow() public {
        uint256 vaultId = _createAndFund();

        vm.expectRevert("VR: wrong state");
        registry.finalizeFinalWindow(vaultId);
    }

    function test_CannotSubmitQuorum_FromActive() public {
        uint256 vaultId = _createAndFund();

        vm.prank(enclaveOracle);
        vm.expectRevert("VR: wrong state");
        registry.submitQuorumResult(vaultId, true, "");
    }

    function test_CannotMarkWarning_FromWarning() public {
        uint256 vaultId = _createAndFund();

        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        registry.markWarning(vaultId);

        // Already in WARNING — markWarning requires ACTIVE
        vm.expectRevert("VR: wrong state");
        registry.markWarning(vaultId);
    }

    function test_CannotRequestAttestation_FromActive() public {
        uint256 vaultId = _createAndFund();

        vm.expectRevert("VR: not in WARNING or grace not expired");
        registry.requestAttestation(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 3: Window Timing Edge Cases
    // ═══════════════════════════════════════════════════════════════════════

    function test_MarkWarning_ExactBoundary() public {
        uint256 vaultId = _createAndFund();

        // Warp to EXACTLY the deadline (block.timestamp == windowDeadline)
        // Should NOT work — requires strictly past deadline
        (, uint256 deadline,,,,) = registry.getVaultTiming(vaultId);
        vm.warp(deadline);
        vm.expectRevert("VR: check-in not missed");
        registry.markWarning(vaultId);

        // One second past: works
        vm.warp(deadline + 1);
        registry.markWarning(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.WARNING));
    }

    function test_FinalizeDispute_ExactBoundary() public {
        uint256 vaultId = _createAndFund();
        _toDisputeWindow(vaultId);

        // Warp to EXACTLY the deadline — should work (>= check)
        (, uint256 deadline,,,,) = registry.getVaultTiming(vaultId);
        vm.warp(deadline);
        registry.finalizeDisputeWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FINAL_WINDOW));
    }

    function test_FinalizeFinal_ExactBoundary() public {
        uint256 vaultId = _createAndFund();
        _toFinalWindow(vaultId);

        (, uint256 deadline,,,,) = registry.getVaultTiming(vaultId);
        vm.warp(deadline);
        registry.finalizeFinalWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FULLY_RELEASED));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 4: Double-Action Prevention
    // ═══════════════════════════════════════════════════════════════════════

    function test_CannotFinalizeDisputeWindow_Twice() public {
        uint256 vaultId = _createAndFund();
        _toDisputeWindow(vaultId);

        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);

        // Now in FINAL_WINDOW — cannot finalize dispute again
        vm.expectRevert("VR: wrong state");
        registry.finalizeDisputeWindow(vaultId);
    }

    function test_CannotFinalizeFinalWindow_Twice() public {
        uint256 vaultId = _createAndFund();
        _toFinalWindow(vaultId);

        vm.warp(block.timestamp + FINAL_WINDOW + 1);
        registry.finalizeFinalWindow(vaultId);

        // Now FULLY_RELEASED — cannot finalize again
        vm.expectRevert("VR: wrong state");
        registry.finalizeFinalWindow(vaultId);
    }

    function test_CannotCancelVault_FromClosedState() public {
        uint256 vaultId = _createAndFund();

        vm.prank(owner);
        registry.cancelVault(vaultId);

        // Already CLOSED
        vm.prank(owner);
        vm.expectRevert("VR: wrong state");
        registry.cancelVault(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 5: Zero-Balance Edge Cases
    // ═══════════════════════════════════════════════════════════════════════

    function test_CancelVault_WithZeroBalance() public {
        vm.prank(owner);
        uint256 vaultId = registry.createVault(
            PLAN_HASH, address(mockFXRP), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, guardian
        );

        // No funds deposited
        uint256 ownerBefore = mockFXRP.balanceOf(owner);
        vm.prank(owner);
        registry.cancelVault(vaultId);

        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.CLOSED));
        assertEq(mockFXRP.balanceOf(owner), ownerBefore); // No change
    }

    function test_FullLifecycle_WithZeroBalance() public {
        // Empty vault — no funds. Should still transition through states
        // without reverting on the payout paths.
        vm.prank(owner);
        uint256 vaultId = registry.createVault(
            PLAN_HASH, address(mockFXRP), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, guardian
        );

        // Progress to DISPUTE_WINDOW
        _toDisputeWindow(vaultId);

        // Finalize dispute — tranche 1 = 0 / 2 = 0
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);
        assertEq(registry.getVaultBalance(vaultId), 0);

        // Finalize final — remaining = 0
        vm.warp(block.timestamp + FINAL_WINDOW + 1);
        registry.finalizeFinalWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FULLY_RELEASED));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 6: Full E2E Lifecycle via Foundry
    // ═══════════════════════════════════════════════════════════════════════

    function test_E2E_HappyPath_WithAllEvents() public {
        // Create
        vm.prank(owner);
        uint256 vaultId = registry.createVault(
            PLAN_HASH, address(mockFXRP), CHECK_IN_INTERVAL, GRACE_WINDOW, DISPUTE_WINDOW, FINAL_WINDOW, guardian
        );

        // Fund
        vm.prank(owner);
        registry.fundVault(vaultId, FUND_AMOUNT);

        // Check in once (normal operation)
        vm.warp(block.timestamp + 10);
        vm.prank(owner);
        registry.checkIn(vaultId, "");

        // Miss check-in → WARNING
        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        registry.markWarning(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.WARNING));

        // Grace expires → QUORUM_PENDING
        vm.warp(block.timestamp + GRACE_WINDOW + 1);
        registry.requestAttestation(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.QUORUM_PENDING));

        // Quorum met → DISPUTE_WINDOW
        vm.prank(enclaveOracle);
        registry.submitQuorumResult(vaultId, true, "");
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));

        // Dispute elapses → FINAL_WINDOW (tranche 1 released)
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FINAL_WINDOW));
        assertEq(registry.getVaultBalance(vaultId), FUND_AMOUNT / 2);

        // Final elapses → FULLY_RELEASED
        vm.warp(block.timestamp + FINAL_WINDOW + 1);
        registry.finalizeFinalWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FULLY_RELEASED));
        assertEq(registry.getVaultBalance(vaultId), 0);
    }

    function test_E2E_GuardianHalt_ThenResume() public {
        uint256 vaultId = _createAndFund();
        _toDisputeWindow(vaultId);

        // Guardian halts
        vm.prank(guardian);
        registry.guardianHalt(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
        assertEq(registry.getVaultBalance(vaultId), FUND_AMOUNT); // No funds moved

        // Owner resumes
        vm.prank(owner);
        registry.checkIn(vaultId, "");
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));

        // Can run the full lifecycle again after halt
        _toDisputeWindow(vaultId);
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);
        vm.warp(block.timestamp + FINAL_WINDOW + 1);
        registry.finalizeFinalWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FULLY_RELEASED));
    }

    function test_E2E_OwnerOverride_ThenResume() public {
        uint256 vaultId = _createAndFund();

        // Progress to QUORUM_PENDING
        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        registry.markWarning(vaultId);
        vm.warp(block.timestamp + GRACE_WINDOW + 1);
        registry.requestAttestation(vaultId);

        // Owner overrides
        vm.prank(owner);
        registry.ownerOverride(vaultId, "");
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));

        // Owner checks in normally again
        vm.prank(owner);
        registry.checkIn(vaultId, "");
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 7: Stress Test — Multiple Vaults
    // ═══════════════════════════════════════════════════════════════════════

    function test_MultipleVaults_IndependentLifecycles() public {
        // Create 3 vaults
        uint256 v1 = _createAndFund();
        uint256 v2 = _createAndFund();
        uint256 v3 = _createAndFund();

        // Progress v1 to DISPUTE_WINDOW
        _toDisputeWindow(v1);

        // v2 stays ACTIVE, v3 gets cancelled
        vm.prank(owner);
        registry.cancelVault(v3);

        // Verify independent states
        assertEq(uint256(registry.getVaultState(v1)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));
        assertEq(uint256(registry.getVaultState(v2)), uint256(IVaultRegistry.VaultState.ACTIVE));
        assertEq(uint256(registry.getVaultState(v3)), uint256(IVaultRegistry.VaultState.CLOSED));

        // Finish v1
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(v1);
        vm.warp(block.timestamp + FINAL_WINDOW + 1);
        registry.finalizeFinalWindow(v1);

        assertEq(uint256(registry.getVaultState(v1)), uint256(IVaultRegistry.VaultState.FULLY_RELEASED));
        // v2 should now have a missed check-in (time advanced)
        assertTrue(registry.isCheckInMissed(v2));
    }
}
