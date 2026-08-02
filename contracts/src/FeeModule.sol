// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IFeeModule} from "./interfaces/IFeeModule.sol";

/**
 * @title FeeModule
 * @notice Phase 2 — interface + comments only.
 * @dev Fee model is a roadmap narrative point, not built this cycle.
 *      Annual plan-maintenance fee funds the check-in infrastructure.
 *      Kept as a compilable stub so the repo layout matches Section 7.
 */
contract FeeModule is IFeeModule {
    function creationFee(address, uint256) external pure returns (uint256) {
        return 0; // No fees in Phase 1
    }

    function annualFee(uint256) external pure returns (uint256) {
        return 0; // No fees in Phase 1
    }

    function collectFees(uint256) external pure {
        revert("FeeModule: not implemented - Phase 2");
    }
}
