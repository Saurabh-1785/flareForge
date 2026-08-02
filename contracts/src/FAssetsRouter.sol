// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IFAssetsRouter} from "./interfaces/IFAssetsRouter.sol";

/**
 * @title FAssetsRouter
 * @author Continuity Vault Team
 * @notice Handles FXRP funding into vaults and native XRP redemption at release.
 *
 * @dev Phase 1: FXRP only. BTC/DOGE are Phase 2.
 *      Design Principle #7: Settle in the asset people understand.
 *      Owners mint FXRP themselves first; this contract only needs ERC-20 transferFrom.
 *      At release, calls FAssets' existing redeem path for native XRP payout.
 *
 *      Full implementation is Layer 2. Scaffold only for Layer 0.
 */
contract FAssetsRouter is IFAssetsRouter {
    // Layer 2 implementation — placeholder for compilation.

    function fundWithFAsset(uint256 /* vaultId */, address /* fAsset */, uint256 /* amount */) external pure {
        revert("FAssetsRouter: not implemented - Layer 2");
    }

    function redeemToNative(
        uint256 /* vaultId */,
        address /* fAsset */,
        uint256 /* amount */,
        string calldata /* destinationAddress */
    ) external pure {
        revert("FAssetsRouter: not implemented - Layer 2");
    }
}
