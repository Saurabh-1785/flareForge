// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

/**
 * @title IAttestorBondRegistry
 * @notice Interface for the bonded attestor market — stake + slash logic.
 * @dev Phase 2 — interface stub only. The bonded attestation market has a cold-start
 *      problem (see Honest Risk Ledger). Phase 1 uses a single, unbonded trustee.
 *      This interface is defined now so the VaultRegistry enum includes SLASHING_REVIEW
 *      for forward compatibility.
 */
interface IAttestorBondRegistry {
    event AttestorRegistered(address indexed attestor, uint256 stake);
    event AttestorSlashed(address indexed attestor, uint256 slashedAmount, uint256 vaultId);
    event StakeWithdrawn(address indexed attestor, uint256 amount);

    /// @notice Register as a bonded attestor by staking collateral.
    function registerAttestor(uint256 stakeAmount) external;

    /// @notice Slash an attestor's stake after a successful dispute.
    /// @dev Only callable by the VaultRegistry during SLASHING_REVIEW resolution.
    function slashAttestor(address attestor, uint256 vaultId) external;

    /// @notice Check if an attestor has sufficient stake to attest.
    function isEligible(address attestor) external view returns (bool);

    /// @notice Get the current stake of an attestor.
    function stakeOf(address attestor) external view returns (uint256);
}
