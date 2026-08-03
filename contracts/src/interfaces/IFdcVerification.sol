// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IWeb2Json} from "./IWeb2Json.sol";

/**
 * @title IFdcVerification
 * @notice Interface for Flare's deployed FdcVerification contract.
 * @dev Verifies Merkle proofs against the on-chain Merkle root stored by FDC.
 *      On Coston2, this is a system contract at a known address.
 *
 *      Reference: https://dev.flare.network/fdc/reference/IFdcVerification
 */
interface IFdcVerification {
    /// @notice Verify a Web2Json attestation proof against the stored Merkle root.
    /// @param _proof The Merkle proof wrapping the Web2Json response.
    /// @return True if the proof is valid.
    function verifyWeb2JsonProof(IWeb2Json.Proof memory _proof) external view returns (bool);
}
