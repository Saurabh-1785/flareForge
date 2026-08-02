// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

/**
 * @title IFeeModule
 * @notice Interface for the fee model — annual plan-maintenance fee, funds check-in infra.
 * @dev Phase 2 — interface + comments only. Fee model is a roadmap narrative point,
 *      not built this cycle. Kept as an interface so the VaultRegistry can reference
 *      it without coupling to a concrete implementation.
 */
interface IFeeModule {
    /// @notice Calculate the fee for creating a new vault.
    /// @return fee The fee amount in the vault's funding asset.
    function creationFee(address fundingAsset, uint256 fundingAmount) external view returns (uint256 fee);

    /// @notice Calculate the annual maintenance fee for a vault.
    /// @return fee The annual fee in the vault's funding asset.
    function annualFee(uint256 vaultId) external view returns (uint256 fee);

    /// @notice Collect accrued fees for a vault.
    function collectFees(uint256 vaultId) external;
}
