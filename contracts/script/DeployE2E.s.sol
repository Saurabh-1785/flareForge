// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {VaultRegistry} from "../src/VaultRegistry.sol";
import {MockERC20} from "../test/mocks/MockERC20.sol";

/**
 * @title DeployE2E
 * @notice Deploys VaultRegistry + MockERC20 for E2E testing.
 *         Sets quorum threshold to 1 for fast testing.
 *
 * @dev Usage:
 *      anvil &
 *      forge script script/DeployE2E.s.sol --rpc-url http://127.0.0.1:8545 --broadcast
 *
 *      Then export the addresses:
 *      export VAULT_REGISTRY_ADDRESS=<logged address>
 *      export FXRP_ADDRESS=<logged address>
 */
contract DeployE2E is Script {
    function run() external {
        // Anvil default key 0
        uint256 deployerKey = vm.envOr(
            "PRIVATE_KEY",
            uint256(0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80)
        );
        // Anvil key 2 = enclave oracle
        address enclaveOracle = vm.envOr(
            "ENCLAVE_ORACLE_ADDRESS",
            address(0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC)
        );

        vm.startBroadcast(deployerKey);

        // Deploy mock FXRP
        MockERC20 fxrp = new MockERC20("Fake FXRP", "FXRP", 18);
        console.log("MockERC20 (FXRP) deployed at:", address(fxrp));

        // Deploy VaultRegistry
        VaultRegistry registry = new VaultRegistry(enclaveOracle);
        console.log("VaultRegistry deployed at:", address(registry));

        // Set quorum threshold to 1 for e2e
        registry.setQuorumThreshold(1);
        console.log("Quorum threshold set to 1");

        // Mint 100k FXRP to deployer (Anvil account 0)
        fxrp.mint(vm.addr(deployerKey), 100_000 ether);
        console.log("Minted 100,000 FXRP to deployer");

        vm.stopBroadcast();

        // Print export commands
        console.log("");
        console.log("=== Copy-paste these exports ===");
        console.log("export VAULT_REGISTRY_ADDRESS=%s", vm.toString(address(registry)));
        console.log("export FXRP_ADDRESS=%s", vm.toString(address(fxrp)));
        console.log("export ENCLAVE_ORACLE_KEY=5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a");
    }
}
