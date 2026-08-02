// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IAttestorBondRegistry} from "./interfaces/IAttestorBondRegistry.sol";

/**
 * @title AttestorBondRegistry
 * @notice Phase 2 — interface stub only.
 * @dev Bonded attestation market has a cold-start problem (Honest Risk Ledger).
 *      Phase 1 uses a single, unbonded trustee via the attestation-api + FDC Web2Json.
 *      SLASHING_REVIEW state exists in VaultRegistry enum for forward compatibility.
 */
contract AttestorBondRegistry is IAttestorBondRegistry {
    function registerAttestor(uint256) external pure {
        revert("AttestorBondRegistry: not implemented - Phase 2");
    }

    function slashAttestor(address, uint256) external pure {
        revert("AttestorBondRegistry: not implemented - Phase 2");
    }

    function isEligible(address) external pure returns (bool) {
        return true; // Phase 1: all attestors are eligible (unbonded)
    }

    function stakeOf(address) external pure returns (uint256) {
        return 0; // No staking in Phase 1
    }
}
