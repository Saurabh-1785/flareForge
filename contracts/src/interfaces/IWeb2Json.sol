// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

/**
 * @title IWeb2Json
 * @notice Flare FDC Web2Json attestation type structs.
 * @dev Matches Flare's Web2Json specification for FDC proof verification.
 *      Web2Json fetches arbitrary Web2 JSON data, applies a JQ transform,
 *      and ABI-encodes the result for on-chain consumption.
 *
 *      Reference: https://dev.flare.network/fdc/attestation-types/web2-json
 */
interface IWeb2Json {
    /// @notice The request body for a Web2Json attestation.
    struct RequestBody {
        /// @notice URL of the data source.
        string url;
        /// @notice HTTP method (GET, POST, etc.).
        string httpMethod;
        /// @notice Request headers as stringified JSON.
        string headers;
        /// @notice Query parameters as stringified JSON.
        string queryParams;
        /// @notice Request body as stringified JSON.
        string body;
        /// @notice JQ filter to postprocess the JSON response.
        string postProcessJq;
        /// @notice ABI signature of the Solidity struct for decoding.
        string abiSignature;
    }

    /// @notice The response body returned by FDC after verification.
    struct ResponseBody {
        /// @notice ABI-encoded data matching the abiSignature.
        bytes abiEncodedData;
    }

    /// @notice Full attestation request.
    struct Request {
        /// @notice Attestation type identifier (bytes32-encoded "Web2Json").
        bytes32 attestationType;
        /// @notice Source identifier (bytes32-encoded "PublicWeb2").
        bytes32 sourceId;
        /// @notice The message integrity code (MIC) for the request.
        bytes32 messageIntegrityCode;
        /// @notice The request body.
        RequestBody requestBody;
    }

    /// @notice Full attestation response.
    struct Response {
        /// @notice The attestation type identifier.
        bytes32 attestationType;
        /// @notice The source identifier.
        bytes32 sourceId;
        /// @notice The voting round in which the request was processed.
        uint64 votingRound;
        /// @notice The lowest-used timestamp for the attestation.
        uint64 lowestUsedTimestamp;
        /// @notice The request body (echoed back).
        RequestBody requestBody;
        /// @notice The response body containing verified data.
        ResponseBody responseBody;
    }

    /// @notice A Merkle proof wrapping a Web2Json response.
    struct Proof {
        /// @notice Merkle proof path.
        bytes32[] merkleProof;
        /// @notice The verified response data.
        Response data;
    }
}
