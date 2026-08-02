// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {VaultRegistry} from "../src/VaultRegistry.sol";
import {FAssetsRouter} from "../src/FAssetsRouter.sol";
import {FeeModule} from "../src/FeeModule.sol";
import {AttestorBondRegistry} from "../src/AttestorBondRegistry.sol";

/**
 * @title Deploy
 * @notice Deploys Continuity Vault contracts to Coston2.
 * @dev Usage:
 *      forge script script/Deploy.s.sol --rpc-url $COSTON2_RPC --broadcast
 *
 *      Set ENCLAVE_ORACLE_ADDRESS env var to the enclave's signing address.
 *      If not set, defaults to the deployer (for local testing only).
 */
contract Deploy is Script {
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("PRIVATE_KEY");
        address enclaveOracle = vm.envOr("ENCLAVE_ORACLE_ADDRESS", vm.addr(deployerPrivateKey));

        vm.startBroadcast(deployerPrivateKey);

        // Core state machine
        VaultRegistry registry = new VaultRegistry(enclaveOracle);
        console.log("VaultRegistry deployed at:", address(registry));

        // Stubs — compilable placeholders for repo layout
        FAssetsRouter router = new FAssetsRouter();
        console.log("FAssetsRouter deployed at:", address(router));

        FeeModule feeModule = new FeeModule();
        console.log("FeeModule deployed at:", address(feeModule));

        AttestorBondRegistry bondRegistry = new AttestorBondRegistry();
        console.log("AttestorBondRegistry deployed at:", address(bondRegistry));

        vm.stopBroadcast();
    }
}
