package chainevent

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

const (
	FairLaunchLocalE2EHTTPUnauthorized = "fair_launch_local_e2e_http_unauthorized"
	fairLaunchLocalHTTPMaxBodyBytes    = 16 << 10
)

var fairLaunchLocalHTTPNoncePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type FairLaunchLocalE2EHTTPOptions struct {
	BindAddress    string
	FrontendOrigin string
	AdminOrigin    string
	Token          string
}

type FairLaunchLocalE2EHTTPAuthority struct {
	Authority      string    `json:"authority"`
	RunID          string    `json:"runId"`
	BaseURL        string    `json:"baseUrl"`
	FrontendOrigin string    `json:"frontendOrigin"`
	AdminOrigin    string    `json:"adminOrigin"`
	Token          string    `json:"token"`
	ExpiresAt      time.Time `json:"expiresAt"`
	Status         string    `json:"status"`
}

type FairLaunchLocalE2EHTTPServer struct {
	listener   net.Listener
	server     *http.Server
	authority  FairLaunchLocalE2EHTTPAuthority
	capability FairLaunchLocalE2ECapability
	ingest     *gorm.DB
	database   *gorm.DB
	mu         sync.Mutex
	nonces     map[string]time.Time
}

type fairLaunchLocalHTTPProject struct {
	ID                   uint64 `json:"id"`
	ProjectID            uint64 `json:"projectID"`
	Name                 string `json:"name"`
	NameCn               string `json:"nameCn"`
	Status               string `json:"status"`
	FundingMode          string `json:"fundingMode"`
	ContractVersion      string `json:"contractVersion"`
	ChainID              uint64 `json:"chainId"`
	ContractAddress      string `json:"contractAddress"`
	TokenAddress         string `json:"tokenAddress"`
	TokenSymbol          string `json:"tokenSymbol"`
	TokenDecimals        uint8  `json:"tokenDecimals"`
	PaymentToken         string `json:"paymentToken"`
	PaymentTokenAddress  string `json:"paymentTokenAddress"`
	PaymentTokenDecimals uint8  `json:"paymentTokenDecimals"`
	SaleTokenAmount      string `json:"saleTokenAmount"`
	SaleTokenAmountRaw   string `json:"saleTokenAmountRaw"`
	SoftCap              string `json:"softCap"`
	SoftCapRaw           string `json:"softCapRaw"`
	TotalRaised          string `json:"totalRaised"`
	TotalRaisedRaw       string `json:"totalRaisedRaw"`
	MinContribution      string `json:"minContribution"`
	MinContributionRaw   string `json:"minContributionRaw"`
	MaxContribution      string `json:"maxContribution"`
	MaxContributionRaw   string `json:"maxContributionRaw"`
	StartTime            uint64 `json:"startTime"`
	EndTime              uint64 `json:"endTime"`
	Description          string `json:"description"`
	Tags                 string `json:"tags"`
	TeamInfo             string `json:"teamInfo"`
	Roadmap              string `json:"roadmap"`
	Partners             string `json:"partners"`
	Tagline              string `json:"tagline"`
	Category             string `json:"category"`
	CoverURL             string `json:"coverUrl"`
	CoverVideoURL        string `json:"coverVideoUrl"`
	StoryHTML            string `json:"storyHtml"`
	ResearchHTML         string `json:"researchHtml"`
	RisksHTML            string `json:"risksHtml"`
	TaglineZh            string `json:"taglineZh"`
	StoryHTMLZh          string `json:"storyHtmlZh"`
	ResearchHTMLZh       string `json:"researchHtmlZh"`
	RisksHTMLZh          string `json:"risksHtmlZh"`
	TeamJSON             string `json:"teamJson"`
	TokenomicsJSON       string `json:"tokenomicsJson"`
	RoadmapJSON          string `json:"roadmapJson"`
	DocumentsJSON        string `json:"documentsJson"`
	FAQJSON              string `json:"faqJson"`
	OnchainJSON          string `json:"onchainJson"`
}

type fairLaunchLocalHTTPFinancialValue struct {
	State   string  `json:"state"`
	Raw     *string `json:"raw"`
	Display *string `json:"display"`
}

type fairLaunchLocalHTTPAction struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitzero"`
}

type fairLaunchLocalHTTPRuntime struct {
	ChainID                uint64 `json:"chainId"`
	ProxyAddress           string `json:"proxyAddress"`
	ImplementationAddress  string `json:"implementationAddress"`
	ImplementationCodeHash string `json:"implementationCodeHash"`
	SourceRegistryRevision uint64 `json:"sourceRegistryRevision"`
	SourceRegistryDigest   string `json:"sourceRegistryDigest"`
	ObservedBlockNumber    string `json:"observedBlockNumber"`
	ObservedBlockHash      string `json:"observedBlockHash"`
	ObservedAt             string `json:"observedAt"`
	RuntimeVerified        bool   `json:"runtimeVerified"`
}

type fairLaunchLocalHTTPPositionResponse struct {
	ProjectID       string `json:"projectId"`
	Wallet          string `json:"wallet"`
	FundingMode     string `json:"fundingMode"`
	ContractVersion string `json:"contractVersion"`
	ChainStatus     string `json:"chainStatus"`
	Assets          struct {
		PaymentToken struct {
			Address  string `json:"address"`
			Decimals uint8  `json:"decimals"`
		} `json:"paymentToken"`
		ProjectToken struct {
			Address  string `json:"address"`
			Decimals uint8  `json:"decimals"`
		} `json:"projectToken"`
	} `json:"assets"`
	Projection struct {
		Contribution        fairLaunchLocalHTTPFinancialValue `json:"contribution"`
		Claimed             fairLaunchLocalHTTPFinancialValue `json:"claimed"`
		Claimable           fairLaunchLocalHTTPFinancialValue `json:"claimable"`
		FinalAllocation     fairLaunchLocalHTTPFinancialValue `json:"finalAllocation"`
		Refund              fairLaunchLocalHTTPFinancialValue `json:"refund"`
		EstimatedAllocation fairLaunchLocalHTTPFinancialValue `json:"estimatedAllocation"`
	} `json:"projection"`
	Actions map[string]fairLaunchLocalHTTPAction `json:"actions"`
	Runtime fairLaunchLocalHTTPRuntime           `json:"runtime"`
	Source  struct {
		StateSource         string `json:"stateSource"`
		Finality            string `json:"finality"`
		ReconciliationState string `json:"reconciliationState"`
	} `json:"source"`
}

type fairLaunchLocalHTTPAdminRuntime struct {
	ChainID                uint64 `json:"chain_id"`
	ContractVersion        string `json:"contract_version"`
	ProxyAddress           string `json:"proxy_address"`
	ImplementationAddress  string `json:"implementation_address"`
	ImplementationCodeHash string `json:"implementation_code_hash"`
	RuntimeVerified        bool   `json:"runtime_verified"`
	SourceRegistryRevision uint64 `json:"source_registry_revision"`
	SourceRegistryDigest   string `json:"source_registry_digest"`
	ObservedAt             string `json:"observed_at"`
	ObservedBlockHash      string `json:"observed_block_hash"`
	ObservedBlockNumber    string `json:"observed_block_number"`
}

type fairLaunchLocalHTTPAdminAction struct {
	Action                       string `json:"action"`
	ApprovalOperation            string `json:"approval_operation"`
	ExecuteRoute                 string `json:"execute_route"`
	Enabled                      bool   `json:"enabled"`
	ExecutionDomain              string `json:"execution_domain"`
	ReasonCode                   string `json:"reason_code,omitzero"`
	RequiresFourEyes             bool   `json:"requires_four_eyes"`
	RequiresPreview              bool   `json:"requires_preview"`
	RequiresStepUpMFA            bool   `json:"requires_step_up_mfa"`
	RequiresThreePartySeparation bool   `json:"requires_three_party_separation"`
	StepUpMFAVerified            bool   `json:"step_up_mfa_verified"`
}

type fairLaunchLocalHTTPAdminCapabilities struct {
	Actions         map[string]fairLaunchLocalHTTPAdminAction `json:"actions"`
	Available       bool                                      `json:"available"`
	ContractVersion string                                    `json:"contract_version"`
	FundingMode     string                                    `json:"funding_mode"`
	FinancialState  map[string]any                            `json:"financial_state"`
	ObservedAt      string                                    `json:"observed_at"`
	PauseState      map[string]any                            `json:"pause_state"`
	ProjectID       uint64                                    `json:"project_id"`
	Runtime         fairLaunchLocalHTTPAdminRuntime           `json:"runtime"`
	RuntimeEnabled  bool                                      `json:"runtime_enabled"`
}

type fairLaunchLocalHTTPApplyRequest struct {
	Nonce           string `json:"nonce"`
	ProjectID       uint64 `json:"projectId"`
	RunID           string `json:"runId"`
	TransactionHash string `json:"transactionHash"`
}

type fairLaunchLocalHTTPApplyResponse struct {
	Applied []FairLaunchLocalE2EAppliedEvidence `json:"applied"`
	Replay  bool                                `json:"replay"`
	Status  string                              `json:"status"`
	TxHash  string                              `json:"transactionHash"`
}

type fairLaunchLocalHTTPAPIError struct {
	Status  bool   `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type fairLaunchLocalHTTPAPIResponse[T any] struct {
	Status bool `json:"status"`
	Data   T    `json:"data"`
}

func NewFairLaunchLocalE2EHTTPServer(capability FairLaunchLocalE2ECapability, ingest, database *gorm.DB, options FairLaunchLocalE2EHTTPOptions, now time.Time) (*FairLaunchLocalE2EHTTPServer, error) {
	if !capability.valid(now.UTC()) || ingest == nil || database == nil {
		return nil, errors.New(FairLaunchLocalE2ECapabilityRequired)
	}
	if options.BindAddress == "" {
		options.BindAddress = "127.0.0.1:0"
	}
	if !fairLaunchLocalExactBindAddress(options.BindAddress) || !fairLaunchLocalExactOrigin(options.FrontendOrigin) || !fairLaunchLocalExactOrigin(options.AdminOrigin) || options.FrontendOrigin == options.AdminOrigin {
		return nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	admission, _ := capability.Admission()
	expiresAt := admission.ExpiresAt.UTC()
	if expiresAt.IsZero() || !now.UTC().Before(expiresAt) {
		return nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	token := strings.TrimSpace(options.Token)
	if token == "" {
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
		}
		token = hex.EncodeToString(raw[:])
	}
	if !fairLaunchLocalDigestPattern.MatchString(token) {
		return nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	listener, err := net.Listen("tcp4", options.BindAddress)
	if err != nil {
		return nil, err
	}
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil || host != "127.0.0.1" {
		_ = listener.Close()
		return nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	baseURL := "http://127.0.0.1:" + port
	runtime := &FairLaunchLocalE2EHTTPServer{
		listener:   listener,
		capability: capability,
		ingest:     ingest,
		database:   database,
		nonces:     map[string]time.Time{},
		authority:  FairLaunchLocalE2EHTTPAuthority{Authority: fairLaunchLocalAuthority, RunID: admission.RunID, BaseURL: baseURL, FrontendOrigin: options.FrontendOrigin, AdminOrigin: options.AdminOrigin, Token: token, ExpiresAt: expiresAt, Status: "LOCAL_ONLY_HTTP_ADMITTED"},
	}
	runtime.server = &http.Server{Handler: runtime, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 190 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 16 << 10}
	return runtime, nil
}

func (server *FairLaunchLocalE2EHTTPServer) Authority() FairLaunchLocalE2EHTTPAuthority {
	return server.authority
}

func (server *FairLaunchLocalE2EHTTPServer) Serve() error {
	if server == nil || server.server == nil || server.listener == nil {
		return errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	err := server.server.Serve(server.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (server *FairLaunchLocalE2EHTTPServer) Shutdown(ctx context.Context) error {
	if server == nil || server.server == nil {
		return nil
	}
	return server.server.Shutdown(ctx)
}

func (server *FairLaunchLocalE2EHTTPServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("cache-control", "no-store")
	writer.Header().Set("x-content-type-options", "nosniff")
	writer.Header().Set("content-security-policy", "default-src 'none'; frame-ancestors 'none'")
	writer.Header().Set("referrer-policy", "no-referrer")
	if request.Host != strings.TrimPrefix(server.authority.BaseURL, "http://") {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	origin := request.Header.Get("origin")
	if origin == "" {
		origin = request.Header.Get("x-jc-fl-origin")
	}
	if origin != server.authority.FrontendOrigin && origin != server.authority.AdminOrigin {
		server.writeError(writer, http.StatusForbidden, FairLaunchLocalE2EHTTPUnauthorized)
		return
	}
	writer.Header().Set("access-control-allow-origin", origin)
	writer.Header().Set("vary", "Origin")
	if request.Method == http.MethodOptions {
		if request.Header.Get("access-control-request-method") == "" {
			server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
			return
		}
		writer.Header().Set("access-control-allow-methods", "GET, POST, OPTIONS")
		writer.Header().Set("access-control-allow-headers", "content-type, x-jc-fl-run-id, x-jc-fl-capability, x-jc-fl-origin")
		writer.Header().Set("access-control-max-age", "60")
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if !server.authorized(request, time.Now().UTC()) {
		server.writeError(writer, http.StatusUnauthorized, FairLaunchLocalE2EHTTPUnauthorized)
		return
	}
	path := request.URL.Path
	if strings.HasPrefix(path, "/admin/") && origin != server.authority.AdminOrigin {
		server.writeError(writer, http.StatusForbidden, FairLaunchLocalE2EHTTPUnauthorized)
		return
	}
	if (strings.HasPrefix(path, "/client/") || strings.HasPrefix(path, "/local/")) && origin != server.authority.FrontendOrigin {
		server.writeError(writer, http.StatusForbidden, FairLaunchLocalE2EHTTPUnauthorized)
		return
	}
	if request.Method == http.MethodGet && path == "/health" {
		server.writeOK(writer, map[string]any{"authority": fairLaunchLocalAuthority, "runId": server.authority.RunID, "status": "ok", "expiresAt": server.authority.ExpiresAt})
		return
	}
	if request.Method == http.MethodGet && path == "/client/project/v1/detail" {
		server.handleProjectDetail(writer, request)
		return
	}
	if request.Method == http.MethodGet && path == "/client/project/v1/list" {
		server.handleProjectList(writer, request)
		return
	}
	if request.Method == http.MethodGet && (path == "/client/project/v1/updates" || path == "/client/project/v1/userStatus") {
		server.handleClientAuxiliaryRead(writer, request)
		return
	}
	if request.Method == http.MethodGet && strings.HasPrefix(path, "/client/project/v1/fair-launch/") && strings.HasSuffix(path, "/position") {
		server.handlePosition(writer, request)
		return
	}
	if request.Method == http.MethodGet && path == "/client/project/v1/fair-launch/portfolio" {
		server.handlePortfolio(writer, request)
		return
	}
	if request.Method == http.MethodPost && path == "/local/v1/transactions/apply" {
		server.handleApply(writer, request)
		return
	}
	if request.Method == http.MethodGet && strings.HasPrefix(path, "/admin/v1/projects/") && strings.HasSuffix(path, "/content") {
		server.handleAdminProject(writer, request)
		return
	}
	if request.Method == http.MethodGet && strings.HasPrefix(path, "/admin/v1/projects/") && strings.HasSuffix(path, "/fair-launch/capabilities") {
		server.handleAdminCapabilities(writer, request)
		return
	}
	if request.Method == http.MethodGet && path == "/admin/v1/my/permissions" {
		server.writeOK(writer, map[string]any{"is_super": false, "permissions": []string{"project.fair_launch.local_e2e"}, "roles": []string{"local_e2e_admin"}})
		return
	}
	server.writeError(writer, http.StatusNotFound, "not_found")
}

func (server *FairLaunchLocalE2EHTTPServer) handleClientAuxiliaryRead(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	if request.URL.Path == "/client/project/v1/updates" {
		if len(query) < 1 || len(query) > 3 || !query.Has("id") {
			server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
			return
		}
		server.writeOK(writer, map[string]any{"items": []any{}, "page": 1, "pageSize": 10, "total": 0})
		return
	}
	if len(query) != 1 || !query.Has("id") {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	server.writeOK(writer, map[string]any{"subscribed": false, "watched": false, "whitelisted": false})
}

func (server *FairLaunchLocalE2EHTTPServer) authorized(request *http.Request, now time.Time) bool {
	if !server.capability.valid(now) || !now.Before(server.authority.ExpiresAt) || request.Header.Get("x-jc-fl-run-id") != server.authority.RunID {
		return false
	}
	supplied := request.Header.Get("x-jc-fl-capability")
	return len(supplied) == len(server.authority.Token) && subtle.ConstantTimeCompare([]byte(supplied), []byte(server.authority.Token)) == 1
}

func fairLaunchLocalExactBindAddress(value string) bool {
	host, port, err := net.SplitHostPort(value)
	if err != nil || host != "127.0.0.1" || port == "" {
		return false
	}
	parsed, err := strconv.Atoi(port)
	return err == nil && parsed >= 0 && parsed <= 65535
}

func fairLaunchLocalExactOrigin(value string) bool {
	parsed, err := exactFairLaunchLocalLoopbackURL(value)
	return err == nil && parsed.Path == ""
}

func (server *FairLaunchLocalE2EHTTPServer) writeError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("content-type", "application/json")
	writer.WriteHeader(status)
	_ = jsonv2.MarshalWrite(writer, fairLaunchLocalHTTPAPIError{Status: false, Code: code, Message: code})
}

func (server *FairLaunchLocalE2EHTTPServer) writeOK(writer http.ResponseWriter, data any) {
	writer.Header().Set("content-type", "application/json")
	_ = jsonv2.MarshalWrite(writer, fairLaunchLocalHTTPAPIResponse[any]{Status: true, Data: data})
}

func (server *FairLaunchLocalE2EHTTPServer) projectIDFromPath(path, prefix, suffix string) (uint64, bool) {
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if raw == "" || strings.Contains(raw, "/") || (len(raw) > 1 && raw[0] == '0') {
		return 0, false
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	return value, err == nil && value > 0
}

func (server *FairLaunchLocalE2EHTTPServer) manifestProject(projectID uint64) (fairLaunchManifestProject, bool) {
	for _, project := range server.capability.seal.manifest.Projects {
		if project.ProjectID == strconv.FormatUint(projectID, 10) {
			return project, true
		}
	}
	return fairLaunchManifestProject{}, false
}

func (server *FairLaunchLocalE2EHTTPServer) projectState(projectID uint64) (entity.FairLaunchProjectBindingV3, entity.FairLaunchProjectStateV3, error) {
	admission := server.capability.seal.admission
	var binding entity.FairLaunchProjectBindingV3
	if err := server.database.Where("run_row_id=? AND source_id=? AND project_id=?", admission.RunRowID, admission.SourceID, projectID).Take(&binding).Error; err != nil {
		return binding, entity.FairLaunchProjectStateV3{}, err
	}
	var state entity.FairLaunchProjectStateV3
	if err := server.database.Where("project_binding_id=? AND run_row_id=? AND project_id=? AND tombstoned=0", binding.ID, admission.RunRowID, projectID).Take(&state).Error; err != nil || !fairLaunchProjectStateDigestValid(state) {
		return binding, state, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	return binding, state, nil
}

func fairLaunchLocalDisplay(raw string, decimals uint8) string {
	if raw == "" {
		return "0"
	}
	if decimals == 0 {
		return raw
	}
	width := int(decimals) + 1
	if len(raw) < width {
		raw = strings.Repeat("0", width-len(raw)) + raw
	}
	whole := raw[:len(raw)-int(decimals)]
	fraction := strings.TrimRight(raw[len(raw)-int(decimals):], "0")
	if fraction == "" {
		return whole
	}
	return whole + "." + fraction
}

func (server *FairLaunchLocalE2EHTTPServer) publicProject(projectID uint64) (fairLaunchLocalHTTPProject, error) {
	manifest, ok := server.manifestProject(projectID)
	if !ok {
		return fairLaunchLocalHTTPProject{}, gorm.ErrRecordNotFound
	}
	binding, state, err := server.projectState(projectID)
	if err != nil {
		return fairLaunchLocalHTTPProject{}, err
	}
	start, err1 := strconv.ParseUint(manifest.StartTime, 10, 64)
	end, err2 := strconv.ParseUint(manifest.EndTime, 10, 64)
	if err1 != nil || err2 != nil {
		return fairLaunchLocalHTTPProject{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	var content entity.Project
	if err := server.database.Where("id=? AND project_id=?", binding.ProjectRowID, projectID).Take(&content).Error; err != nil {
		return fairLaunchLocalHTTPProject{}, err
	}
	if content.ChainID != binding.ChainID || content.ContractAddress != binding.ContractAddress || content.ContractVersion != fairLaunchLocalContractVersion || content.FundingMode != fairLaunchLocalFundingMode || !content.OnchainLocked || content.TokenAddress != binding.ProjectTokenAddress || content.PaymentTokenAddress != binding.PaymentTokenAddress || content.StoryHTML == "" || content.ResearchHTML == "" || content.RisksHTML == "" || content.TeamJSON == "" || content.TokenomicsJSON == "" || content.RoadmapJSON == "" || content.DocumentsJSON == "" || content.FAQJSON == "" {
		return fairLaunchLocalHTTPProject{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	tokenSymbol := ""
	for _, token := range server.capability.seal.manifest.Assets.ProjectTokens {
		if token.Address == binding.ProjectTokenAddress {
			tokenSymbol = token.Symbol
		}
	}
	return fairLaunchLocalHTTPProject{ID: projectID, ProjectID: projectID, Name: content.NameEn, NameCn: content.NameCn, Status: state.Status, FundingMode: fairLaunchLocalFundingMode, ContractVersion: fairLaunchLocalContractVersion, ChainID: binding.ChainID, ContractAddress: binding.ContractAddress, TokenAddress: binding.ProjectTokenAddress, TokenSymbol: tokenSymbol, TokenDecimals: binding.ProjectTokenDecimals, PaymentToken: server.capability.seal.manifest.Assets.PaymentToken.Symbol, PaymentTokenAddress: binding.PaymentTokenAddress, PaymentTokenDecimals: binding.PaymentTokenDecimals, SaleTokenAmount: fairLaunchLocalDisplay(state.SaleTokenAmountRaw, binding.ProjectTokenDecimals), SaleTokenAmountRaw: state.SaleTokenAmountRaw, SoftCap: fairLaunchLocalDisplay(manifest.SoftCapRaw, binding.PaymentTokenDecimals), SoftCapRaw: manifest.SoftCapRaw, TotalRaised: fairLaunchLocalDisplay(state.TotalRaisedRaw, binding.PaymentTokenDecimals), TotalRaisedRaw: state.TotalRaisedRaw, MinContribution: fairLaunchLocalDisplay(manifest.MinContributionRaw, binding.PaymentTokenDecimals), MinContributionRaw: manifest.MinContributionRaw, MaxContribution: fairLaunchLocalDisplay(manifest.MaxContributionRaw, binding.PaymentTokenDecimals), MaxContributionRaw: manifest.MaxContributionRaw, StartTime: start, EndTime: end, Description: content.Description, Tags: content.Tags, TeamInfo: content.TeamInfo, Roadmap: content.Roadmap, Partners: content.Partners, Tagline: content.Tagline, Category: content.Category, CoverURL: content.CoverURL, CoverVideoURL: content.CoverVideoURL, StoryHTML: content.StoryHTML, ResearchHTML: content.ResearchHTML, RisksHTML: content.RisksHTML, TaglineZh: content.TaglineZh, StoryHTMLZh: content.StoryHTMLZh, ResearchHTMLZh: content.ResearchHTMLZh, RisksHTMLZh: content.RisksHTMLZh, TeamJSON: content.TeamJSON, TokenomicsJSON: content.TokenomicsJSON, RoadmapJSON: content.RoadmapJSON, DocumentsJSON: content.DocumentsJSON, FAQJSON: content.FAQJSON, OnchainJSON: content.OnchainJSON}, nil
}

func (server *FairLaunchLocalE2EHTTPServer) handleProjectDetail(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Query().Has("projectID") == false || len(request.URL.Query()) != 1 {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	projectID, err := strconv.ParseUint(request.URL.Query().Get("projectID"), 10, 64)
	if err != nil || projectID == 0 {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	project, err := server.publicProject(projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		server.writeError(writer, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	server.writeOK(writer, project)
}

func (server *FairLaunchLocalE2EHTTPServer) handleProjectList(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	if !query.Has("cursor") || len(query) < 1 || len(query) > 2 || query.Get("cursor") != "0" {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	status := strings.TrimSpace(query.Get("status"))
	if status != "" && !regexp.MustCompile(`^[A-Za-z_]{1,32}$`).MatchString(status) {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	projects := make([]fairLaunchLocalHTTPProject, 0, len(server.capability.seal.manifest.Projects))
	for _, manifestProject := range server.capability.seal.manifest.Projects {
		projectID, err := strconv.ParseUint(manifestProject.ProjectID, 10, 64)
		if err != nil || projectID == 0 {
			server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
			return
		}
		project, err := server.publicProject(projectID)
		if err != nil {
			server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
			return
		}
		if status == "" || strings.EqualFold(project.Status, status) {
			projects = append(projects, project)
		}
	}
	server.writeOK(writer, map[string]any{"cursor": 0, "list": projects})
}

func (server *FairLaunchLocalE2EHTTPServer) positionResponse(projectID uint64, wallet string) (fairLaunchLocalHTTPPositionResponse, error) {
	manifest, ok := server.manifestProject(projectID)
	if !ok {
		return fairLaunchLocalHTTPPositionResponse{}, gorm.ErrRecordNotFound
	}
	binding, state, err := server.projectState(projectID)
	if err != nil {
		return fairLaunchLocalHTTPPositionResponse{}, err
	}
	var position entity.FairLaunchPositionV3
	positionErr := server.database.Where("project_binding_id=? AND project_id=? AND wallet_address=? AND tombstoned=0", binding.ID, projectID, wallet).Take(&position).Error
	if positionErr != nil && !errors.Is(positionErr, gorm.ErrRecordNotFound) {
		return fairLaunchLocalHTTPPositionResponse{}, positionErr
	}
	if positionErr == nil && !fairLaunchPositionStateDigestValid(position) {
		return fairLaunchLocalHTTPPositionResponse{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	contributionRaw, claimedRaw, refunded := "0", "0", false
	if positionErr == nil {
		contributionRaw, claimedRaw, refunded = position.ContributionAmountRaw, position.ClaimedTokensRaw, position.Refunded
	}
	finalAllocationRaw := "0"
	if state.Status == "Successful" && state.FinalTotalRaisedRaw != nil && *state.FinalTotalRaisedRaw != "0" {
		allocation, allocationErr := fairLaunchLocalMulDiv(contributionRaw, state.SaleTokenAmountRaw, *state.FinalTotalRaisedRaw)
		if allocationErr != nil {
			return fairLaunchLocalHTTPPositionResponse{}, allocationErr
		}
		finalAllocationRaw = allocation
	}
	claimableRaw := "0"
	if state.Status == "Successful" {
		claimed, okClaimed := new(big.Int).SetString(claimedRaw, 10)
		allocation, okAllocation := new(big.Int).SetString(finalAllocationRaw, 10)
		if !okClaimed || !okAllocation {
			return fairLaunchLocalHTTPPositionResponse{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
		}
		if allocation.Cmp(claimed) >= 0 {
			claimableRaw = new(big.Int).Sub(allocation, claimed).String()
		}
	}
	estimatedRaw := "0"
	if state.TotalRaisedRaw != "0" {
		estimatedRaw, _ = fairLaunchLocalMulDiv(contributionRaw, state.SaleTokenAmountRaw, state.TotalRaisedRaw)
	}
	value := func(raw string, decimals uint8) fairLaunchLocalHTTPFinancialValue {
		display := fairLaunchLocalDisplay(raw, decimals)
		return fairLaunchLocalHTTPFinancialValue{State: "available", Raw: &raw, Display: &display}
	}
	refundRaw := "0"
	if refunded {
		refundRaw = contributionRaw
	}
	response := fairLaunchLocalHTTPPositionResponse{ProjectID: strconv.FormatUint(projectID, 10), Wallet: wallet, FundingMode: fairLaunchLocalFundingMode, ContractVersion: fairLaunchLocalContractVersion, ChainStatus: state.Status, Actions: map[string]fairLaunchLocalHTTPAction{}}
	response.Assets.PaymentToken.Address, response.Assets.PaymentToken.Decimals = binding.PaymentTokenAddress, binding.PaymentTokenDecimals
	response.Assets.ProjectToken.Address, response.Assets.ProjectToken.Decimals = binding.ProjectTokenAddress, binding.ProjectTokenDecimals
	response.Projection.Contribution = value(contributionRaw, binding.PaymentTokenDecimals)
	response.Projection.Claimed = value(claimedRaw, binding.ProjectTokenDecimals)
	response.Projection.Claimable = value(claimableRaw, binding.ProjectTokenDecimals)
	response.Projection.FinalAllocation = value(finalAllocationRaw, binding.ProjectTokenDecimals)
	response.Projection.Refund = value(refundRaw, binding.PaymentTokenDecimals)
	response.Projection.EstimatedAllocation = value(estimatedRaw, binding.ProjectTokenDecimals)
	response.Actions["contribute"] = fairLaunchLocalHTTPAction{Available: state.Status == "Active", Reason: fairLaunchLocalActionReason(state.Status == "Active")}
	response.Actions["claim"] = fairLaunchLocalHTTPAction{Available: state.Status == "Successful" && claimableRaw != "0", Reason: fairLaunchLocalActionReason(state.Status == "Successful" && claimableRaw != "0")}
	response.Actions["refund"] = fairLaunchLocalHTTPAction{Available: (state.Status == "Failed" || state.Status == "Cancelled") && contributionRaw != "0" && !refunded, Reason: fairLaunchLocalActionReason((state.Status == "Failed" || state.Status == "Cancelled") && contributionRaw != "0" && !refunded)}
	observedAt := server.capability.seal.manifest.CreatedAt
	response.Runtime = fairLaunchLocalHTTPRuntime{ChainID: binding.ChainID, ProxyAddress: binding.ContractAddress, ImplementationAddress: server.capability.seal.manifest.Contracts["dispatcher"].Address, ImplementationCodeHash: server.capability.seal.manifest.Contracts["dispatcher"].RuntimeCodeHash, SourceRegistryRevision: server.capability.seal.admission.SourceRevisionID, SourceRegistryDigest: "0x" + server.capability.seal.admission.SourceRegistryDigest, ObservedBlockNumber: strconv.FormatUint(server.capability.seal.admission.SignedTipBlock, 10), ObservedBlockHash: server.capability.seal.admission.SignedTipHash, ObservedAt: observedAt, RuntimeVerified: true}
	response.Source.StateSource, response.Source.Finality, response.Source.ReconciliationState = "canonical_chain_event", "finalized", "balanced"
	_ = manifest
	return response, nil
}

func fairLaunchLocalActionReason(available bool) string {
	if available {
		return ""
	}
	return "chain_state_unavailable"
}

func fairLaunchLocalMulDiv(left, right, denominator string) (string, error) {
	a, okA := new(big.Int).SetString(left, 10)
	b, okB := new(big.Int).SetString(right, 10)
	c, okC := new(big.Int).SetString(denominator, 10)
	if !okA || !okB || !okC || a.Sign() < 0 || b.Sign() < 0 || c.Sign() <= 0 {
		return "", errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	return new(big.Int).Div(new(big.Int).Mul(a, b), c).String(), nil
}

func (server *FairLaunchLocalE2EHTTPServer) handlePosition(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := server.projectIDFromPath(request.URL.Path, "/client/project/v1/fair-launch/", "/position")
	query := request.URL.Query()
	wallet := strings.ToLower(query.Get("wallet"))
	if !ok || len(query) != 1 || !query.Has("wallet") || !fairLaunchLocalAddressPattern.MatchString(wallet) {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	response, err := server.positionResponse(projectID, wallet)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		server.writeError(writer, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	server.writeOK(writer, response)
}

func (server *FairLaunchLocalE2EHTTPServer) handlePortfolio(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	wallet := strings.ToLower(query.Get("wallet"))
	if len(query) != 1 || !query.Has("wallet") || !fairLaunchLocalAddressPattern.MatchString(wallet) {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	positions := make([]fairLaunchLocalHTTPPositionResponse, 0, 3)
	for _, project := range server.capability.seal.manifest.Projects {
		projectID, _ := strconv.ParseUint(project.ProjectID, 10, 64)
		position, err := server.positionResponse(projectID, wallet)
		if err != nil {
			server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
			return
		}
		positions = append(positions, position)
	}
	server.writeOK(writer, map[string]any{"positions": positions, "claims": []any{}, "refunds": []any{}})
}

func (server *FairLaunchLocalE2EHTTPServer) handleApply(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("content-type") != "application/json" {
		server.writeError(writer, http.StatusUnsupportedMediaType, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, fairLaunchLocalHTTPMaxBodyBytes)
	defer request.Body.Close()
	var payload fairLaunchLocalHTTPApplyRequest
	decoderRaw, err := io.ReadAll(request.Body)
	if err != nil || len(decoderRaw) == 0 || len(decoderRaw) > fairLaunchLocalHTTPMaxBodyBytes || jsonv2.Unmarshal(decoderRaw, &payload, jsonv2.RejectUnknownMembers(true)) != nil {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	if payload.RunID != server.authority.RunID || !fairLaunchLocalHTTPNoncePattern.MatchString(payload.Nonce) || !fairLaunchLocalHashPattern.MatchString(payload.TransactionHash) {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	if _, ok := server.manifestProject(payload.ProjectID); !ok {
		server.writeError(writer, http.StatusForbidden, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	server.mu.Lock()
	if _, exists := server.nonces[payload.Nonce]; exists {
		server.mu.Unlock()
		server.writeError(writer, http.StatusConflict, "fair_launch_local_e2e_replay_rejected")
		return
	}
	server.nonces[payload.Nonce] = time.Now().UTC()
	server.mu.Unlock()
	ctx, cancel := context.WithTimeout(request.Context(), 180*time.Second)
	defer cancel()
	writes, err := LoadFairLaunchLocalE2ETransaction(ctx, server.capability, payload.TransactionHash)
	if err != nil || len(writes) != 1 || writes[0].Event.ProjectID == nil || !writes[0].Event.ProjectID.IsUint64() || writes[0].Event.ProjectID.Uint64() != payload.ProjectID {
		server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	result, err := ProcessFairLaunchLocalE2E(server.capability, server.ingest, server.database, writes[0], time.Now().UTC())
	if err != nil {
		server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	evidence, err := VerifyFairLaunchLocalE2EApplied(server.capability, server.database, result)
	if err != nil {
		server.writeError(writer, http.StatusConflict, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	server.writeOK(writer, fairLaunchLocalHTTPApplyResponse{Applied: []FairLaunchLocalE2EAppliedEvidence{evidence}, Replay: result.Effect.Disposition == FairLaunchEffectNoOp, Status: "APPLIED_AND_VERIFIED", TxHash: payload.TransactionHash})
}

func (server *FairLaunchLocalE2EHTTPServer) handleAdminProject(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := server.projectIDFromPath(request.URL.Path, "/admin/v1/projects/", "/content")
	if !ok || len(request.URL.Query()) != 0 {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	project, err := server.publicProject(projectID)
	if err != nil {
		server.writeError(writer, http.StatusNotFound, "not_found")
		return
	}
	server.writeOK(writer, map[string]any{"id": project.ID, "project_id": project.ProjectID, "name_en": project.Name, "name_cn": project.NameCn, "tagline": project.Tagline, "description": project.Description, "tags": project.Tags, "status": project.Status, "funding_mode": project.FundingMode, "contract_version": project.ContractVersion, "onchain_locked": true, "category": project.Category, "cover_url": project.CoverURL, "cover_video_url": project.CoverVideoURL, "launch_at": nil, "story_html": project.StoryHTML, "research_html": project.ResearchHTML, "risks_html": project.RisksHTML, "tagline_zh": project.TaglineZh, "story_html_zh": project.StoryHTMLZh, "research_html_zh": project.ResearchHTMLZh, "risks_html_zh": project.RisksHTMLZh, "team_json": project.TeamJSON, "tokenomics_json": project.TokenomicsJSON, "roadmap_json": project.RoadmapJSON, "documents_json": project.DocumentsJSON, "faq_json": project.FAQJSON, "onchain_json": project.OnchainJSON, "sale_token_amount": project.SaleTokenAmount, "sale_token_amount_raw": project.SaleTokenAmountRaw, "soft_cap": project.SoftCap, "soft_cap_raw": project.SoftCapRaw, "created_at": server.capability.seal.manifest.CreatedAt, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (server *FairLaunchLocalE2EHTTPServer) adminAction(action string, enabled bool) fairLaunchLocalHTTPAdminAction {
	reason := ""
	if !enabled {
		reason = "chain_state_unavailable"
	}
	return fairLaunchLocalHTTPAdminAction{Action: action, ApprovalOperation: "fair_launch.local_e2e." + action, ExecuteRoute: "/local/v1/transactions/apply", Enabled: enabled, ExecutionDomain: "operator_durable_outbox", ReasonCode: reason, RequiresFourEyes: false, RequiresPreview: false, RequiresStepUpMFA: false, RequiresThreePartySeparation: false, StepUpMFAVerified: true}
}

func (server *FairLaunchLocalE2EHTTPServer) handleAdminCapabilities(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := server.projectIDFromPath(request.URL.Path, "/admin/v1/projects/", "/fair-launch/capabilities")
	if !ok || len(request.URL.Query()) != 0 {
		server.writeError(writer, http.StatusBadRequest, FairLaunchLocalE2EAdmissionRejected)
		return
	}
	binding, state, err := server.projectState(projectID)
	if err != nil {
		server.writeError(writer, http.StatusNotFound, "not_found")
		return
	}
	manifest, _ := server.manifestProject(projectID)
	actions := map[string]fairLaunchLocalHTTPAdminAction{
		"end":                            server.adminAction("end", state.Status == "Active"),
		"cancel":                         server.adminAction("cancel", state.Status == "Active" || state.Status == "Pending"),
		"release":                        server.adminAction("release", state.Status == "SuccessfulPendingRelease"),
		"withdraw_project_proceeds":      server.adminAction("withdraw_project_proceeds", state.Status == "Successful" && !state.ProjectProceedsWithdrawn),
		"withdraw_platform_fee":          server.adminAction("withdraw_platform_fee", state.Status == "Successful" && !state.PlatformFeeWithdrawn),
		"withdraw_failed_project_tokens": server.adminAction("withdraw_failed_project_tokens", (state.Status == "Failed" || state.Status == "Cancelled") && !state.FailedProjectTokensWithdrawn),
	}
	observedAt := time.Now().UTC().Format(time.RFC3339Nano)
	capabilities := fairLaunchLocalHTTPAdminCapabilities{Actions: actions, Available: true, ContractVersion: fairLaunchLocalContractVersion, FundingMode: fairLaunchLocalFundingMode, FinancialState: map[string]any{"sale_token_amount": fairLaunchLocalDisplay(state.SaleTokenAmountRaw, binding.ProjectTokenDecimals), "sale_token_amount_raw": state.SaleTokenAmountRaw, "soft_cap": fairLaunchLocalDisplay(manifest.SoftCapRaw, binding.PaymentTokenDecimals), "soft_cap_raw": manifest.SoftCapRaw, "final_total_raised": fairLaunchLocalDisplay(fairLaunchLocalString(state.FinalTotalRaisedRaw), binding.PaymentTokenDecimals), "final_total_raised_raw": fairLaunchLocalString(state.FinalTotalRaisedRaw), "platform_fee_withdrawn": state.PlatformFeeWithdrawn, "project_proceeds_withdrawn": state.ProjectProceedsWithdrawn, "reconciliation_state": "balanced", "deposited_sale_tokens": fairLaunchLocalDisplay(state.TotalTokensDepositedRaw, binding.ProjectTokenDecimals), "deposited_sale_tokens_raw": state.TotalTokensDepositedRaw, "required_deposit": fairLaunchLocalDisplay(state.SaleTokenAmountRaw, binding.ProjectTokenDecimals), "required_deposit_raw": state.SaleTokenAmountRaw}, ObservedAt: observedAt, PauseState: map[string]any{"claims_paused": false, "contributions_paused": false, "payouts_paused": false}, ProjectID: projectID, RuntimeEnabled: true}
	capabilities.Runtime = fairLaunchLocalHTTPAdminRuntime{ChainID: binding.ChainID, ContractVersion: fairLaunchLocalContractVersion, ProxyAddress: binding.ContractAddress, ImplementationAddress: server.capability.seal.manifest.Contracts["dispatcher"].Address, ImplementationCodeHash: server.capability.seal.manifest.Contracts["dispatcher"].RuntimeCodeHash, RuntimeVerified: true, SourceRegistryRevision: server.capability.seal.admission.SourceRevisionID, SourceRegistryDigest: "0x" + server.capability.seal.admission.SourceRegistryDigest, ObservedAt: server.capability.seal.manifest.CreatedAt, ObservedBlockHash: server.capability.seal.admission.SignedTipHash, ObservedBlockNumber: strconv.FormatUint(server.capability.seal.admission.SignedTipBlock, 10)}
	server.writeOK(writer, capabilities)
}

func fairLaunchLocalString(value *string) string {
	if value == nil {
		return "0"
	}
	return *value
}
