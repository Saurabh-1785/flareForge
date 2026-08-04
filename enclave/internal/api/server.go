// Package api provides the REST API for the Vault Enclave service.
// These are the endpoints the relayer (Layer 4) and the owner's client
// interact with.
//
// Endpoints (matching the build prompt specification):
//   POST /vaults/{id}/plan           — accepts encrypted plan blob + commitment hash
//   POST /vaults/{id}/attestations   — accepts a verified FDC attestation reference
//   GET  /vaults/{id}/quorum-status  — polled by the relayer
//   GET  /vaults/{id}/quorum-result  — returns signed result for on-chain submission
//   POST /vaults/{id}/reset          — resets quorum (e.g., after guardian halt)
//   GET  /health                     — health + enclave identity info
//   GET  /identity                   — enclave public keys + attestation evidence
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	enclavecrypto "github.com/continuity-vault/enclave/internal/crypto"
	"github.com/continuity-vault/enclave/internal/quorum"
	"github.com/continuity-vault/enclave/internal/sealedstore"
)

// Server is the Vault Enclave REST API server.
type Server struct {
	router  *mux.Router
	store   *sealedstore.Store
	engine  *quorum.Engine
	keys    *enclavecrypto.EnclaveKeys
	logger  *zap.Logger
	startAt time.Time

	// attestationEvidence is the remote attestation proof from the TEE platform.
	// Set by the TEE bootstrapper on startup.
	attestationEvidence string
}

// Config holds server configuration.
type Config struct {
	Port                int
	AttestationEvidence string
	Logger              *zap.Logger
}

// NewServer creates a new API server with all dependencies wired.
func NewServer(
	db *sql.DB,
	keys *enclavecrypto.EnclaveKeys,
	cfg Config,
) (*Server, error) {
	logger := cfg.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	store, err := sealedstore.NewStore(db, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to create sealed store: %w", err)
	}

	engine, err := quorum.NewEngine(db, store, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to create quorum engine: %w", err)
	}

	s := &Server{
		store:               store,
		engine:              engine,
		keys:                keys,
		logger:              logger,
		startAt:             time.Now().UTC(),
		attestationEvidence: cfg.AttestationEvidence,
	}

	s.setupRoutes()
	return s, nil
}

// Router returns the configured HTTP router for use in http.ListenAndServe.
func (s *Server) Router() http.Handler {
	return s.router
}

// setupRoutes configures all API routes.
func (s *Server) setupRoutes() {
	s.router = mux.NewRouter()

	// Middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)

	// Health & identity
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/identity", s.handleIdentity).Methods("GET")

	// Vault-specific endpoints
	s.router.HandleFunc("/vaults/{id}/plan", s.handleStorePlan).Methods("POST")
	s.router.HandleFunc("/vaults/{id}/plan", s.handleGetPlanMetadata).Methods("GET")
	s.router.HandleFunc("/vaults/{id}/attestations", s.handleSubmitAttestation).Methods("POST")
	s.router.HandleFunc("/vaults/{id}/quorum-status", s.handleQuorumStatus).Methods("GET")
	s.router.HandleFunc("/vaults/{id}/quorum-result", s.handleQuorumResult).Methods("GET")
	s.router.HandleFunc("/vaults/{id}/reset", s.handleResetQuorum).Methods("POST")
}

// ──────────────────────────────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────────────────────────────

// POST /vaults/{id}/plan — accepts the encrypted plan blob + commitment hash
func (s *Server) handleStorePlan(w http.ResponseWriter, r *http.Request) {
	vaultID, err := s.parseVaultID(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid vault ID")
		return
	}

	var sub sealedstore.PlanSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if sub.CommitmentHash == "" {
		s.respondError(w, http.StatusBadRequest, "commitmentHash is required")
		return
	}
	if len(sub.PlanData) == 0 {
		s.respondError(w, http.StatusBadRequest, "planData is required")
		return
	}

	if err := s.store.StorePlan(vaultID, sub); err != nil {
		s.logger.Error("failed to store plan",
			zap.Uint64("vaultId", vaultID),
			zap.Error(err),
		)
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.logger.Info("plan sealed",
		zap.Uint64("vaultId", vaultID),
		zap.String("commitmentHash", sub.CommitmentHash),
	)

	s.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success":        true,
		"vaultId":        vaultID,
		"commitmentHash": sub.CommitmentHash,
		"message":        "Plan sealed in enclave. Commitment hash links to on-chain pointer.",
	})
}

// GET /vaults/{id}/plan — returns plan metadata (NOT the plan itself — never exposed)
func (s *Server) handleGetPlanMetadata(w http.ResponseWriter, r *http.Request) {
	vaultID, err := s.parseVaultID(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid vault ID")
		return
	}

	meta, err := s.store.GetPlanMetadata(vaultID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, meta)
}

// POST /vaults/{id}/attestations — accepts a verified FDC attestation reference
func (s *Server) handleSubmitAttestation(w http.ResponseWriter, r *http.Request) {
	vaultID, err := s.parseVaultID(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid vault ID")
		return
	}

	var ref quorum.AttestationRef
	if err := json.NewDecoder(r.Body).Decode(&ref); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Override vault ID from URL path (URL is authoritative)
	ref.VaultID = vaultID

	if ref.CaseID == "" {
		s.respondError(w, http.StatusBadRequest, "caseId is required")
		return
	}
	if ref.AttestationType == "" {
		s.respondError(w, http.StatusBadRequest, "attestationType is required")
		return
	}
	if ref.AttestorAddress == "" {
		s.respondError(w, http.StatusBadRequest, "attestorAddress is required")
		return
	}

	status, err := s.engine.SubmitAttestation(ref)
	if err != nil {
		s.logger.Error("failed to submit attestation",
			zap.Uint64("vaultId", vaultID),
			zap.String("attestor", ref.AttestorAddress),
			zap.Error(err),
		)
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.logger.Info("attestation received",
		zap.Uint64("vaultId", vaultID),
		zap.String("attestor", ref.AttestorAddress),
		zap.String("type", ref.AttestationType),
		zap.Bool("quorumMet", status.QuorumMet),
		zap.Int("count", status.CurrentCount),
	)

	s.respondJSON(w, http.StatusOK, status)
}

// GET /vaults/{id}/quorum-status — polled by the relayer
func (s *Server) handleQuorumStatus(w http.ResponseWriter, r *http.Request) {
	vaultID, err := s.parseVaultID(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid vault ID")
		return
	}

	status, err := s.engine.GetQuorumStatus(vaultID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, status)
}

// GET /vaults/{id}/quorum-result — returns signed result for on-chain submission
func (s *Server) handleQuorumResult(w http.ResponseWriter, r *http.Request) {
	vaultID, err := s.parseVaultID(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid vault ID")
		return
	}

	payload, err := s.engine.GetQuorumResultPayload(vaultID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, payload)
}

// POST /vaults/{id}/reset — resets quorum data (e.g., after guardian halt)
func (s *Server) handleResetQuorum(w http.ResponseWriter, r *http.Request) {
	vaultID, err := s.parseVaultID(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid vault ID")
		return
	}

	if err := s.engine.ResetQuorum(vaultID); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("quorum reset", zap.Uint64("vaultId", vaultID))

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"vaultId": vaultID,
		"message": "Quorum data reset. New attestations required.",
	})
}

// GET /health — health check with enclave identity
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"service":   "continuity-vault-enclave",
		"startedAt": s.startAt.Format(time.RFC3339),
		"uptime":    time.Since(s.startAt).String(),
		"enclave": map[string]interface{}{
			"signingAddress": s.keys.SigningAddress(),
			"teeProvider":    "Google Cloud Confidential Space",
		},
	})
}

// GET /identity — full enclave public keys + attestation evidence
func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	info := s.keys.GetKeyInfo(s.attestationEvidence)
	s.respondJSON(w, http.StatusOK, info)
}

// ──────────────────────────────────────────────────────────────────────
// Middleware
// ──────────────────────────────────────────────────────────────────────

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────

func (s *Server) parseVaultID(r *http.Request) (uint64, error) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		return 0, fmt.Errorf("missing vault ID")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid vault ID: %s", idStr)
	}
	if id == 0 {
		return 0, fmt.Errorf("vault ID must be > 0")
	}
	return id, nil
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) respondError(w http.ResponseWriter, status int, message string) {
	s.respondJSON(w, status, map[string]interface{}{
		"error":   true,
		"message": message,
	})
}
