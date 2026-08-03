// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {FAssetsRouter} from "../src/FAssetsRouter.sol";
import {MockERC20} from "./mocks/MockERC20.sol";

/**
 * @title FAssetsRouterTest
 * @notice Layer 2 test suite for FAssetsRouter:
 *         1. Fund vaults with allowlisted FAsset (FXRP)
 *         2. Redeem to native (event emission for MVP)
 *         3. Allowlisting and access control
 *         4. Edge cases and reverts
 */
contract FAssetsRouterTest is Test {
    FAssetsRouter public router;
    MockERC20 public mockFXRP;
    MockERC20 public mockFBTC;

    address public vaultRegistry = makeAddr("vaultRegistry");
    address public deployer = makeAddr("deployer");
    address public user = makeAddr("user");
    address public attacker = makeAddr("attacker");

    function setUp() public {
        mockFXRP = new MockERC20("Fake FXRP", "FXRP", 18);
        mockFBTC = new MockERC20("Fake FBTC", "FBTC", 8);

        vm.prank(deployer);
        router = new FAssetsRouter(vaultRegistry, address(mockFXRP));

        // Fund user with tokens
        mockFXRP.mint(user, 10_000 ether);
        mockFBTC.mint(user, 100e8);

        // User approves router
        vm.prank(user);
        mockFXRP.approve(address(router), type(uint256).max);
        vm.prank(user);
        mockFBTC.approve(address(router), type(uint256).max);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 1: Constructor
    // ═══════════════════════════════════════════════════════════════════════

    function test_Constructor() public view {
        assertEq(router.vaultRegistry(), vaultRegistry);
        assertTrue(router.allowlistedFAssets(address(mockFXRP)));
        assertFalse(router.allowlistedFAssets(address(mockFBTC)));
    }

    function test_Constructor_RevertZeroRegistry() public {
        vm.prank(deployer);
        vm.expectRevert("FAR: zero registry");
        new FAssetsRouter(address(0), address(mockFXRP));
    }

    function test_Constructor_RevertZeroFxrp() public {
        vm.prank(deployer);
        vm.expectRevert("FAR: zero fxrp");
        new FAssetsRouter(vaultRegistry, address(0));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 2: Allowlisting
    // ═══════════════════════════════════════════════════════════════════════

    function test_AllowlistFAsset() public {
        vm.prank(deployer);
        router.allowlistFAsset(address(mockFBTC));
        assertTrue(router.allowlistedFAssets(address(mockFBTC)));
    }

    function test_AllowlistFAsset_RevertNotAdmin() public {
        vm.prank(attacker);
        vm.expectRevert("FAR: not admin");
        router.allowlistFAsset(address(mockFBTC));
    }

    function test_AllowlistFAsset_RevertZeroAddress() public {
        vm.prank(deployer);
        vm.expectRevert("FAR: zero address");
        router.allowlistFAsset(address(0));
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 3: Funding
    // ═══════════════════════════════════════════════════════════════════════

    function test_FundWithFAsset() public {
        uint256 amount = 500 ether;

        vm.prank(user);
        router.fundWithFAsset(1, address(mockFXRP), amount);

        // Tokens should be at the vault registry
        assertEq(mockFXRP.balanceOf(vaultRegistry), amount);
        assertEq(mockFXRP.balanceOf(user), 10_000 ether - amount);
    }

    function test_FundWithFAsset_EmitsEvent() public {
        vm.prank(user);
        vm.expectEmit(true, false, false, true);
        emit FAssetsRouter.FAssetFunded(1, address(mockFXRP), 500 ether);
        router.fundWithFAsset(1, address(mockFXRP), 500 ether);
    }

    function test_FundWithFAsset_RevertNotAllowlisted() public {
        vm.prank(user);
        vm.expectRevert("FAR: FAsset not allowlisted");
        router.fundWithFAsset(1, address(mockFBTC), 100e8);
    }

    function test_FundWithFAsset_RevertZeroAmount() public {
        vm.prank(user);
        vm.expectRevert("FAR: zero amount");
        router.fundWithFAsset(1, address(mockFXRP), 0);
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 4: Redemption
    // ═══════════════════════════════════════════════════════════════════════

    function test_RedeemToNative() public {
        uint256 amount = 200 ether;

        vm.prank(user);
        router.redeemToNative(1, address(mockFXRP), amount, "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh");

        // Tokens should be held by the router (locked for redemption)
        assertEq(mockFXRP.balanceOf(address(router)), amount);
    }

    function test_RedeemToNative_EmitsEvent() public {
        vm.prank(user);
        vm.expectEmit(true, false, false, true);
        emit FAssetsRouter.RedemptionRequested(
            1,
            address(mockFXRP),
            200 ether,
            "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
        );
        router.redeemToNative(1, address(mockFXRP), 200 ether, "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh");
    }

    function test_RedeemToNative_RevertNotAllowlisted() public {
        vm.prank(user);
        vm.expectRevert("FAR: FAsset not allowlisted");
        router.redeemToNative(1, address(mockFBTC), 100e8, "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh");
    }

    function test_RedeemToNative_RevertZeroAmount() public {
        vm.prank(user);
        vm.expectRevert("FAR: zero amount");
        router.redeemToNative(1, address(mockFXRP), 0, "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh");
    }

    function test_RedeemToNative_RevertEmptyDestination() public {
        vm.prank(user);
        vm.expectRevert("FAR: empty destination");
        router.redeemToNative(1, address(mockFXRP), 200 ether, "");
    }

    // ═══════════════════════════════════════════════════════════════════════
    // Test Group 5: Phase 2 - Multiple FAssets
    // ═══════════════════════════════════════════════════════════════════════

    function test_MultipleFAssets_FundAfterAllowlist() public {
        // Allowlist FBTC
        vm.prank(deployer);
        router.allowlistFAsset(address(mockFBTC));

        // Fund with FBTC
        vm.prank(user);
        router.fundWithFAsset(1, address(mockFBTC), 50e8);

        assertEq(mockFBTC.balanceOf(vaultRegistry), 50e8);
    }
}
