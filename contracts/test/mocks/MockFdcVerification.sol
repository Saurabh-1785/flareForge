// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IWeb2Json} from "../../src/interfaces/IWeb2Json.sol";
import {IFdcVerification} from "../../src/interfaces/IFdcVerification.sol";

/**
 * @title MockFdcVerification
 * @notice Mock FDC verification contract for unit testing.
 * @dev Always returns the configured response (default: true).
 *      Toggle with `setShouldVerify(false)` to test rejection paths.
 */
contract MockFdcVerification is IFdcVerification {
    bool public shouldVerify = true;

    function setShouldVerify(bool _shouldVerify) external {
        shouldVerify = _shouldVerify;
    }

    function verifyWeb2JsonProof(IWeb2Json.Proof memory) external view override returns (bool) {
        return shouldVerify;
    }
}
