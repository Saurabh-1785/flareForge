// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {VaultRegistry} from "../src/VaultRegistry.sol";
import {FAssetsRouter} from "../src/FAssetsRouter.sol";
import {FeeModule} from "../src/FeeModule.sol";
import {AttestorBondRegistry} from "../src/AttestorBondRegistry.sol";
import {FdcAttestationVerifier} from "../src/FdcAttestationVerifier.sol";

/**
 * @title Deploy
 * @notice Deploys Continuity Vault contracts to Coston2.
 * @dev Usage:
 *      forge script script/Deploy.s.sol --rpc-url $COSTON2_RPC --broadcast
 *
 *      Required env vars:
 *        PRIVATE_KEY              — deployer private key
 *        ENCLAVE_ORACLE_ADDRESS   — enclave signing address (defaults to deployer)
 *        FXRP_ADDRESS             — FXRP ERC-20 token address on Coston2
 *        FDC_VERIFICATION_ADDRESS — Flare FdcVerification contract on Coston2
 */
contract Deploy is Script {
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("PRIVATE_KEY");
        address enclaveOracle = vm.envOr("ENCLAVE_ORACLE_ADDRESS", vm.addr(deployerPrivateKey));
        address fxrpAddress = vm.envOr("FXRP_ADDRESS", vm.addr(deployerPrivateKey)); // placeholder if not set
        address fdcVerificationAddress = vm.envOr("FDC_VERIFICATION_ADDRESS", vm.addr(deployerPrivateKey));

        vm.startBroadcast(deployerPrivateKey);

        // Core state machine
        VaultRegistry registry = new VaultRegistry(enclaveOracle);
        console.log("VaultRegistry deployed at:", address(registry));

        // FDC attestation verifier (Layer 2)
        FdcAttestationVerifier verifier = new FdcAttestationVerifier(fdcVerificationAddress, address(registry));
        console.log("FdcAttestationVerifier deployed at:", address(verifier));

        // Link verifier to registry
        registry.setFdcVerifier(address(verifier));
        console.log("FdcVerifier linked to VaultRegistry");

        // FAssets router (Layer 2)
        FAssetsRouter router = new FAssetsRouter(address(registry), fxrpAddress);
        console.log("FAssetsRouter deployed at:", address(router));

        // Stubs - Phase 2
        FeeModule feeModule = new FeeModule();
        console.log("FeeModule deployed at:", address(feeModule));

        AttestorBondRegistry bondRegistry = new AttestorBondRegistry();
        console.log("AttestorBondRegistry deployed at:", address(bondRegistry));

        vm.stopBroadcast();
    }
}
