// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {FdcAttestationVerifier} from "../src/FdcAttestationVerifier.sol";
import {VaultRegistry} from "../src/VaultRegistry.sol";
import {IVaultRegistry} from "../src/interfaces/IVaultRegistry.sol";
import {IWeb2Json} from "../src/interfaces/IWeb2Json.sol";
import {MockFdcVerification} from "./mocks/MockFdcVerification.sol";
import {MockERC20} from "./mocks/MockERC20.sol";

/**
 * @title FdcAttestationVerifierTest
 * @notice Layer 2 test suite for the FDC attestation flow:
 *         1. Register a case for a vault
 *         2. Submit a verified FDC Web2Json proof
 *         3. Verify attestation is recorded and dedup works
 *         4. Full integration: FDC attestation quorum -> state transition
 */
contract FdcAttestationVerifierTest is Test {
    FdcAttestationVerifier public verifier;
    VaultRegistry public registry;
    MockFdcVerification public mockFdc;
    MockERC20 public mockFXRP;

    address public owner = makeAddr("owner");
    address public guardian = makeAddr("guardian");
    address public enclaveOracle = makeAddr("enclaveOracle");
    address public relayer = makeAddr("relayer");
    address public trustee1 = makeAddr("trustee1");
    address public trustee2 = makeAddr("trustee2");
    address public attacker = makeAddr("attacker");

    // Timing
    uint256 constant CHECK_IN_INTERVAL = 300;
    uint256 constant GRACE_WINDOW = 120;
    uint256 constant DISPUTE_WINDOW = 180;
    uint256 constant FINAL_WINDOW = 120;

    bytes32 constant PLAN_HASH = keccak256("sealed-plan-v1");
    uint256 constant FUND_AMOUNT = 1000 ether;

    function setUp() public {
        // Deploy mock FDC verification
        mockFdc = new MockFdcVerification();

        // Deploy registry and verifier
        registry = new VaultRegistry(enclaveOracle);
        verifier = new FdcAttestationVerifier(address(mockFdc), address(registry));

        // Link verifier to registry
        registry.setFdcVerifier(address(verifier));

        // Deploy mock token
        mockFXRP = new MockERC20("Fake FXRP", "FXRP", 18);
        mockFXRP.mint(owner, 10_000 ether);
        vm.prank(owner);
        mockFXRP.approve(address(registry), type(uint256).max);
    }

    // ─── Helpers ─────────────────────────────────────────────────────────

    function _createAndFundVault() internal returns (uint256 vaultId) {
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
        vm.prank(owner);
        registry.fundVault(vaultId, FUND_AMOUNT);
    }

    function _buildProof(
        string memory caseId,
        string memory attestationType,
        address attestor
    ) internal pure returns (IWeb2Json.Proof memory) {
        // Encode attestation data matching FdcAttestationVerifier.AttestationData
        bytes memory abiEncodedData = abi.encode(
            FdcAttestationVerifier.AttestationData({
                caseId: caseId,
                attestationType: attestationType,
                attestorAddress: attestor,
                attestedAt: uint256(1700000000),
                confirmed: true
            })
        );

        IWeb2Json.RequestBody memory reqBody = IWeb2Json.RequestBody({
            url: "http://localhost:3000/attestations/case-001",
            httpMethod: "GET",
            headers: "{}",
            queryParams: "{}",
            body: "{}",
            postProcessJq: "{caseId: .caseId, attestationType: .attestationType, attestorAddress: .attestorAddress, attestedAt: .attestedAt, confirmed: .confirmed}",
            abiSignature: '{"components":[{"internalType":"string","name":"caseId","type":"string"},{"internalType":"string","name":"attestationType","type":"string"},{"internalType":"address","name":"attestorAddress","type":"address"},{"internalType":"uint256","name":"attestedAt","type":"uint256"},{"internalType":"bool","name":"confirmed","type":"bool"}],"name":"task","type":"tuple"}'
        });

        IWeb2Json.ResponseBody memory resBody = IWeb2Json.ResponseBody({
            abiEncodedData: abiEncodedData
        });

        IWeb2Json.Response memory response = IWeb2Json.Response({
            attestationType: bytes32("Web2Json"),
            sourceId: bytes32("PublicWeb2"),
            votingRound: 42,
            lowestUsedTimestamp: 0,
            requestBody: reqBody,
            responseBody: resBody
        });

        bytes32[] memory merkleProof = new bytes32[](0);

        return IWeb2Json.Proof({
            merkleProof: merkleProof,
            data: response
        });
    }

    function _advanceToQuorumPending(uint256 vaultId) internal {
        // Miss check-in
        vm.warp(block.timestamp + CHECK_IN_INTERVAL + 1);
        vm.prank(relayer);
        registry.markWarning(vaultId);

        // Grace expires
        vm.warp(block.timestamp + GRACE_WINDOW + 1);
        vm.prank(relayer);
        registry.requestAttestation(vaultId);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 1: Case Registration
    // ═══════════════════════════════════════════════════════════════════════

    function test_RegisterCase() public {
        verifier.registerCase(1, "case-001");
        bytes32 caseHash = keccak256(abi.encodePacked("case-001"));
        assertEq(verifier.caseIdToVault(caseHash), 1);
    }

    function test_RegisterCase_RevertDuplicate() public {
        verifier.registerCase(1, "case-001");
        vm.expectRevert("FAV: case already registered");
        verifier.registerCase(2, "case-001");
    }

    function test_RegisterCase_RevertZeroVault() public {
        vm.expectRevert("FAV: invalid vault ID");
        verifier.registerCase(0, "case-001");
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 2: Attestation Submission
    // ═══════════════════════════════════════════════════════════════════════

    function test_SubmitAttestation() public {
        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");

        IWeb2Json.Proof memory proof = _buildProof("case-001", "DEATH", trustee1);
        verifier.submitAttestation(proof);

        assertEq(verifier.getAttestationCount(vaultId), 1);
        assertFalse(verifier.isQuorumMet(vaultId, 2));
    }

    function test_SubmitAttestation_RevertInvalidProof() public {
        mockFdc.setShouldVerify(false);

        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");

        IWeb2Json.Proof memory proof = _buildProof("case-001", "DEATH", trustee1);
        vm.expectRevert("FAV: invalid FDC proof");
        verifier.submitAttestation(proof);
    }

    function test_SubmitAttestation_RevertDuplicateAttestor() public {
        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");

        IWeb2Json.Proof memory proof = _buildProof("case-001", "DEATH", trustee1);
        verifier.submitAttestation(proof);

        vm.expectRevert("FAV: already attested");
        verifier.submitAttestation(proof);
    }

    function test_SubmitAttestation_RevertUnknownCase() public {
        IWeb2Json.Proof memory proof = _buildProof("unknown-case", "DEATH", trustee1);
        vm.expectRevert("FAV: unknown case ID");
        verifier.submitAttestation(proof);
    }

    function test_SubmitAttestation_MultipleAttestors() public {
        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");

        // Trustee 1 attests
        IWeb2Json.Proof memory proof1 = _buildProof("case-001", "DEATH", trustee1);
        verifier.submitAttestation(proof1);
        assertEq(verifier.getAttestationCount(vaultId), 1);

        // Trustee 2 attests
        IWeb2Json.Proof memory proof2 = _buildProof("case-001", "DEATH", trustee2);
        verifier.submitAttestation(proof2);
        assertEq(verifier.getAttestationCount(vaultId), 2);

        assertTrue(verifier.isQuorumMet(vaultId, 2));
    }

    function test_SubmitAttestation_EmitsEvent() public {
        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");
        bytes32 caseHash = keccak256(abi.encodePacked("case-001"));

        IWeb2Json.Proof memory proof = _buildProof("case-001", "DEATH", trustee1);

        vm.expectEmit(true, true, false, true);
        emit FdcAttestationVerifier.AttestationVerified(vaultId, caseHash, trustee1, 42);
        verifier.submitAttestation(proof);
    }

    function test_GetAttestation() public {
        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");

        IWeb2Json.Proof memory proof = _buildProof("case-001", "DEATH", trustee1);
        verifier.submitAttestation(proof);

        FdcAttestationVerifier.VerifiedAttestation memory att = verifier.getAttestation(vaultId, 0);
        assertEq(att.attestor, trustee1);
        assertEq(att.votingRound, 42);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 3: Full Integration — FDC Attestation -> VaultRegistry State
    // ═══════════════════════════════════════════════════════════════════════

    function test_Integration_FdcQuorumTransitionsVault() public {
        // 1. Create & fund vault
        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.ACTIVE));

        // 2. Advance vault to QUORUM_PENDING
        _advanceToQuorumPending(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.QUORUM_PENDING));

        // 3. First attestation — not enough for quorum
        IWeb2Json.Proof memory proof1 = _buildProof("case-001", "DEATH", trustee1);
        verifier.submitAttestation(proof1);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.QUORUM_PENDING));

        // 4. Second attestation — quorum met, auto-transition to DISPUTE_WINDOW
        IWeb2Json.Proof memory proof2 = _buildProof("case-001", "DEATH", trustee2);
        verifier.submitAttestation(proof2);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));
    }

    function test_Integration_FullLifecycleWithFdc() public {
        // 1. Create, fund, register case
        uint256 vaultId = _createAndFundVault();
        verifier.registerCase(vaultId, "case-001");

        // 2. Miss check-in -> WARNING -> QUORUM_PENDING
        _advanceToQuorumPending(vaultId);

        // 3. Two FDC attestations -> DISPUTE_WINDOW
        verifier.submitAttestation(_buildProof("case-001", "DEATH", trustee1));
        verifier.submitAttestation(_buildProof("case-001", "DEATH", trustee2));
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.DISPUTE_WINDOW));

        // 4. Dispute window elapses -> TRANCHE_1 -> FINAL_WINDOW
        vm.warp(block.timestamp + DISPUTE_WINDOW + 1);
        registry.finalizeDisputeWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FINAL_WINDOW));
        assertEq(registry.getVaultBalance(vaultId), FUND_AMOUNT / 2);

        // 5. Final window elapses -> FULLY_RELEASED
        vm.warp(block.timestamp + FINAL_WINDOW + 1);
        registry.finalizeFinalWindow(vaultId);
        assertEq(uint256(registry.getVaultState(vaultId)), uint256(IVaultRegistry.VaultState.FULLY_RELEASED));
        assertEq(registry.getVaultBalance(vaultId), 0);
    }
}
