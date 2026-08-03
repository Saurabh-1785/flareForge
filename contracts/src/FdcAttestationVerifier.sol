// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IWeb2Json} from "./interfaces/IWeb2Json.sol";
import {IFdcVerification} from "./interfaces/IFdcVerification.sol";

/// @notice Minimal callback interface for VaultRegistry attestation notification.
interface IVaultRegistryCallback {
    function submitVerifiedAttestation(uint256 vaultId) external;
}

/**
 * @title FdcAttestationVerifier
 * @author Continuity Vault Team
 * @notice Accepts FDC Web2Json proofs, verifies them via Flare's FdcVerification,
 *         extracts trustee attestation data, and records verified attestations.
 *
 * @dev This is the bridge between off-chain trustee attestations and the on-chain
 *      Vault Registry state machine.
 *
 *      Flow:
 *      1. Trustee submits attestation to attestation-api/ (off-chain)
 *      2. FDC verifiers fetch the attestation-api response via Web2Json
 *      3. FDC produces a Merkle root stored on-chain
 *      4. Anyone (relayer) submits the Merkle proof here
 *      5. This contract verifies the proof and records the attestation
 *      6. VaultRegistry checks attestation count for quorum
 *
 *      Design Principle #1: Trust-minimize the trigger — multiple independent
 *      attestations required, not a single signal.
 */
contract FdcAttestationVerifier {
    // ─── Types ───────────────────────────────────────────────────────────

    /// @notice Decoded attestation data from the Web2Json response.
    /// @dev Must match the ABI signature used in the FDC Web2Json request.
    struct AttestationData {
        string caseId;
        string attestationType; // e.g., "DEATH", "INCAPACITATION"
        address attestorAddress;
        uint256 attestedAt;     // UNIX timestamp from the API
        bool confirmed;
    }

    /// @notice A recorded, verified attestation.
    struct VerifiedAttestation {
        bytes32 caseIdHash;     // keccak256(caseId) for efficient lookup
        address attestor;
        uint256 verifiedAt;     // block.timestamp when proof was verified
        uint64 votingRound;     // FDC voting round
    }

    // ─── Events ──────────────────────────────────────────────────────────

    event AttestationVerified(
        uint256 indexed vaultId,
        bytes32 indexed caseIdHash,
        address attestor,
        uint64 votingRound
    );

    // ─── Storage ─────────────────────────────────────────────────────────

    /// @notice The Flare FdcVerification contract.
    IFdcVerification public immutable fdcVerification;

    /// @notice The VaultRegistry this verifier feeds attestations into.
    address public immutable vaultRegistry;

    /// @notice Vault ID -> list of verified attestations.
    mapping(uint256 => VerifiedAttestation[]) public vaultAttestations;

    /// @notice Vault ID -> attestor address -> already attested (dedup).
    mapping(uint256 => mapping(address => bool)) public hasAttested;

    /// @notice Vault ID -> caseId hash -> vault mapping (for linking).
    mapping(bytes32 => uint256) public caseIdToVault;

    // ─── Constructor ─────────────────────────────────────────────────────

    /// @param _fdcVerification Address of Flare's FdcVerification contract on Coston2.
    /// @param _vaultRegistry Address of the VaultRegistry contract.
    constructor(address _fdcVerification, address _vaultRegistry) {
        require(_fdcVerification != address(0), "FAV: zero fdc");
        require(_vaultRegistry != address(0), "FAV: zero registry");
        fdcVerification = IFdcVerification(_fdcVerification);
        vaultRegistry = _vaultRegistry;
    }

    // ─── Core ────────────────────────────────────────────────────────────

    /// @notice Register a case ID for a specific vault (called by relayer/owner).
    /// @dev Must be called before attestations can be linked to a vault.
    function registerCase(uint256 vaultId, string calldata caseId) external {
        bytes32 caseHash = keccak256(abi.encodePacked(caseId));
        require(caseIdToVault[caseHash] == 0, "FAV: case already registered");
        require(vaultId > 0, "FAV: invalid vault ID");
        caseIdToVault[caseHash] = vaultId;
    }

    /// @notice Submit a verified FDC Web2Json proof containing a trustee attestation.
    /// @dev Anyone can call this (permissionless proof submission — relayer or trustee).
    ///      The proof is verified against Flare's FdcVerification contract.
    /// @param _proof The FDC Merkle proof wrapping the Web2Json response.
    function submitAttestation(IWeb2Json.Proof memory _proof) external {
        // 1. Verify the Merkle proof against Flare's on-chain root
        require(
            fdcVerification.verifyWeb2JsonProof(_proof),
            "FAV: invalid FDC proof"
        );

        // 2. Decode the ABI-encoded attestation data from the response
        AttestationData memory attestation = abi.decode(
            _proof.data.responseBody.abiEncodedData,
            (AttestationData)
        );

        // 3. Validate the attestation
        require(attestation.confirmed, "FAV: attestation not confirmed");

        // 4. Look up the vault for this case
        bytes32 caseHash = keccak256(abi.encodePacked(attestation.caseId));
        uint256 vaultId = caseIdToVault[caseHash];
        require(vaultId > 0, "FAV: unknown case ID");

        // 5. Dedup: each attestor can only attest once per vault
        require(!hasAttested[vaultId][attestation.attestorAddress], "FAV: already attested");
        hasAttested[vaultId][attestation.attestorAddress] = true;

        // 6. Record the verified attestation
        vaultAttestations[vaultId].push(VerifiedAttestation({
            caseIdHash: caseHash,
            attestor: attestation.attestorAddress,
            verifiedAt: block.timestamp,
            votingRound: _proof.data.votingRound
        }));

        // 7. Notify VaultRegistry (triggers auto-transition if quorum met)
        IVaultRegistryCallback(vaultRegistry).submitVerifiedAttestation(vaultId);

        emit AttestationVerified(
            vaultId,
            caseHash,
            attestation.attestorAddress,
            _proof.data.votingRound
        );
    }

    // ─── View Helpers ────────────────────────────────────────────────────

    /// @notice Get the number of verified attestations for a vault.
    function getAttestationCount(uint256 vaultId) external view returns (uint256) {
        return vaultAttestations[vaultId].length;
    }

    /// @notice Check if quorum is met for a vault (MVP: 2 attestations).
    /// @param vaultId The vault to check.
    /// @param requiredCount The quorum threshold.
    function isQuorumMet(uint256 vaultId, uint256 requiredCount) external view returns (bool) {
        return vaultAttestations[vaultId].length >= requiredCount;
    }

    /// @notice Get a specific attestation for a vault.
    function getAttestation(uint256 vaultId, uint256 index)
        external
        view
        returns (VerifiedAttestation memory)
    {
        require(index < vaultAttestations[vaultId].length, "FAV: index out of bounds");
        return vaultAttestations[vaultId][index];
    }
}
