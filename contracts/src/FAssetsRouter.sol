// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IERC20} from "forge-std/interfaces/IERC20.sol";
import {IFAssetsRouter} from "./interfaces/IFAssetsRouter.sol";

/**
 * @title FAssetsRouter
 * @author Continuity Vault Team
 * @notice Routes FXRP ERC-20 tokens into vaults (funding) and initiates
 *         FAssets redemption at release for native XRP payout.
 *
 * @dev Design Principle #7: Settle in the asset people understand.
 *      - Funding: Owner has already minted FXRP themselves. This contract
 *        does a standard ERC-20 transferFrom into the VaultRegistry.
 *      - Redemption: At TRANCHE_1_RELEASED and FULLY_RELEASED, this contract
 *        calls FAssets' AssetManager.redeem() to initiate a native XRP payout.
 *        The beneficiary receives native XRP on XRPL via FAssets' redemption
 *        window timing (not instant on screen).
 *
 *      Phase 1: FXRP only. BTC/DOGE are Phase 2.
 *      MVP: Redemption uses a mock/interface since real AssetManager agent
 *           selection on Coston2 may not have live agents.
 */
contract FAssetsRouter is IFAssetsRouter {
    // ─── Events ──────────────────────────────────────────────────────────

    event FAssetFunded(uint256 indexed vaultId, address fAsset, uint256 amount);
    event RedemptionRequested(
        uint256 indexed vaultId,
        address fAsset,
        uint256 amount,
        string destinationAddress
    );

    // ─── Storage ─────────────────────────────────────────────────────────

    /// @notice The VaultRegistry contract address.
    address public immutable vaultRegistry;

    /// @notice Allowlisted FAsset token addresses (FXRP for Phase 1).
    mapping(address => bool) public allowlistedFAssets;

    /// @notice Admin address (deployer) for allowlisting.
    address public admin;

    // ─── Constructor ─────────────────────────────────────────────────────

    /// @param _vaultRegistry The VaultRegistry contract address.
    /// @param _fxrp The FXRP ERC-20 token address to allowlist.
    constructor(address _vaultRegistry, address _fxrp) {
        require(_vaultRegistry != address(0), "FAR: zero registry");
        require(_fxrp != address(0), "FAR: zero fxrp");
        vaultRegistry = _vaultRegistry;
        admin = msg.sender;
        allowlistedFAssets[_fxrp] = true;
    }

    // ─── Modifiers ───────────────────────────────────────────────────────

    modifier onlyAdmin() {
        require(msg.sender == admin, "FAR: not admin");
        _;
    }

    // ─── Admin ───────────────────────────────────────────────────────────

    /// @notice Add an FAsset token to the allowlist (Phase 2: BTC, DOGE).
    function allowlistFAsset(address fAsset) external onlyAdmin {
        require(fAsset != address(0), "FAR: zero address");
        allowlistedFAssets[fAsset] = true;
    }

    // ─── Core ────────────────────────────────────────────────────────────

    /// @notice Fund a vault with an allowlisted FAsset (FXRP).
    /// @dev Caller must have approved this contract for `amount`.
    ///      Tokens are transferred to the VaultRegistry for custody.
    function fundWithFAsset(
        uint256 vaultId,
        address fAsset,
        uint256 amount
    ) external {
        require(allowlistedFAssets[fAsset], "FAR: FAsset not allowlisted");
        require(amount > 0, "FAR: zero amount");

        // Pull tokens from caller
        IERC20(fAsset).transferFrom(msg.sender, vaultRegistry, amount);

        emit FAssetFunded(vaultId, fAsset, amount);
    }

    /// @notice Initiate FAssets redemption for native XRP payout.
    /// @dev In production, this calls AssetManager.redeem() which triggers
    ///      the FAssets redemption flow. The beneficiary receives native XRP
    ///      on XRPL after the FAssets redemption window elapses.
    ///
    ///      MVP: Emits an event with the redemption request details.
    ///      The real AssetManager integration requires agent selection and
    ///      specific collateral pool parameters that depend on live Coston2 state.
    ///
    ///      Roadmap: Once Protocol-Managed Wallets extend beyond protocol-scoped
    ///      custody, PMW replaces this redemption step entirely (direct native
    ///      XRP payout without the FAssets round-trip).
    function redeemToNative(
        uint256 vaultId,
        address fAsset,
        uint256 amount,
        string calldata destinationAddress
    ) external {
        require(allowlistedFAssets[fAsset], "FAR: FAsset not allowlisted");
        require(amount > 0, "FAR: zero amount");
        require(bytes(destinationAddress).length > 0, "FAR: empty destination");

        // In production: IERC20(fAsset).approve(assetManager, amount);
        //                IAssetManager(assetManager).redeem(lots, destinationAddress, ...);
        //
        // For MVP, we burn/lock the tokens and emit the redemption request.
        // The relayer watches this event and narrates the expected native payout.
        IERC20(fAsset).transferFrom(msg.sender, address(this), amount);

        emit RedemptionRequested(vaultId, fAsset, amount, destinationAddress);
    }
}
