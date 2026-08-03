// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

/**
 * @title IFAssetsRouter
 * @notice Interface for FAssets funding and native-asset redemption.
 * @dev Phase 1 supports FXRP only. BTC/DOGE via FAssets redemption is Phase 2.
 *      Principle #7: settle in the asset people understand - native XRP, not wrapped.
 */
interface IFAssetsRouter {
    /// @notice Accept FXRP funding into a vault via ERC-20 transferFrom.
    function fundWithFAsset(uint256 vaultId, address fAsset, uint256 amount) external;

    /// @notice Initiate FAssets redemption to pay a beneficiary in native XRP.
    /// @param destinationAddress The native XRPL address of the beneficiary.
    function redeemToNative(uint256 vaultId, address fAsset, uint256 amount, string calldata destinationAddress) external;
}
