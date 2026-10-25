package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"career107/domain"
)

// ---------------------------------------------------------------- registry
//
// Only registered capabilities can be invoked, and every handler is bound to a
// contract from the domain. The registry is infrastructure: it holds no career
// semantics, it only routes a validated request to the handler that declared
// it.

type CapabilityRunContext struct {
	RuntimeID string
	Plan      ExecutionPlan
	Request   domain.CapabilityRequest
	// Digest-verified payload produced at S_3.
	Payload any
}

type CapabilityRunOutput struct {
	Value           any
	OutputRefs      []string
	EvidenceRefs    []string
	UncertaintyRefs []string
	DeclaredLosses  []string
}

type CapabilityHandler interface {
	Contract() domain.CapabilityContract
	Run(ctx CapabilityRunContext) (CapabilityRunOutput, []domain.Rejection)
}

type CapabilityRegistry struct {
	mu       sync.RWMutex
	order    []string
	handlers map[string]CapabilityHandler
}

func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{handlers: map[string]CapabilityHandler{}}
}

// Register refuses handlers whose contract is not declared by the domain, or
// whose contract version drifted from the declaration.
func (r *CapabilityRegistry) Register(handler CapabilityHandler) error {
	contract := handler.Contract()
	if !domain.IsAllowlistedCapability(contract.ID) {
		return fmt.Errorf("refusing to register capability %q: it is not declared in the 107 capability contracts", contract.ID)
	}
	declared, ok := domain.LookupCapabilityContract(contract.ID)
	if !ok || declared.Version != contract.Version {
		return fmt.Errorf("refusing to register capability %q: handler contract version does not match the domain declaration", contract.ID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[contract.ID]; !exists {
		r.order = append(r.order, contract.ID)
	}
	r.handlers[contract.ID] = handler
	return nil
}

func (r *CapabilityRegistry) Get(capabilityID string) (CapabilityHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[capabilityID]
	return handler, ok
}

func (r *CapabilityRegistry) List() []domain.CapabilityContract {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.CapabilityContract, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.handlers[id].Contract())
	}
	return out
}

func (r *CapabilityRegistry) Resolve(capabilityID string) (CapabilityHandler, []domain.Rejection) {
	if !domain.IsAllowlistedCapability(capabilityID) {
		return nil, []domain.Rejection{domain.NewRejection("CAPABILITY.NOT-ALLOWLISTED", fmt.Sprintf("capability %q is not allowlisted", capabilityID), "capabilityId")}
	}
	handler, ok := r.Get(capabilityID)
	if !ok {
		return nil, []domain.Rejection{domain.NewRejection("CAPABILITY.UNREGISTERED", fmt.Sprintf("capability %q has no registered handler", capabilityID), "capabilityId")}
	}
	return handler, nil
}

// ----------------------------------------------------------- authorization
//
//	Capability → Authorization → S_1 → S_2 → Execution → S_4
//
// Authorization is decided before execution and recorded, so an unauthorized
// invocation is a refusal with a reason rather than a failure discovered later
// (§9, §17). The evaluator is pure: given grants, a request, and a contract,
// it returns a decision. Consumption of budget is explicit and separate.

const (
	AuthzPolicyRef     = "policy.authz.capability-allowlist.v1"
	AuthzPolicyVersion = "1.0.0"
)

type AuthorizationDecision struct {
	ID                string          `json:"id"`
	Outcome           string          `json:"outcome"`
	CapabilityID      string          `json:"capabilityId"`
	CapabilityVersion string          `json:"capabilityVersion"`
	Actor             domain.ActorRef `json:"actor"`
	SubjectID         string          `json:"subjectId"`
	RequiredScopes    []string        `json:"requiredScopes"`
	GrantedScopes     []string        `json:"grantedScopes"`
	DecidedAt         string          `json:"decidedAt"`
	Reasons           []string        `json:"reasons"`
	DenialReason      string          `json:"denialReason,omitempty"`
	BudgetRemaining   *int            `json:"budgetRemaining,omitempty"`
	PolicyRef         string          `json:"policyRef"`
	GrantRef          string          `json:"grantRef,omitempty"`
}

type IssueGrantParams struct {
	Actor         domain.ActorRef
	Scopes        []string
	SubjectIDs    []string
	IssuedAt      string
	ExpiresAt     string
	Budget        map[string]int
	MaxInputBytes *int
}

type EvaluateParams struct {
	Request    domain.CapabilityRequest
	Contract   domain.CapabilityContract
	InputBytes int
	DecidedAt  string
}

type AuthorizationService struct {
	mu      sync.Mutex
	grants  map[string]domain.AuthorizationGrant
	order   []string
	budgets map[string]map[string]int
}

func NewAuthorizationService(grants ...domain.AuthorizationGrant) *AuthorizationService {
	s := &AuthorizationService{grants: map[string]domain.AuthorizationGrant{}, budgets: map[string]map[string]int{}}
	for _, grant := range grants {
		s.AddGrant(grant)
	}
	return s
}

func (s *AuthorizationService) AddGrant(grant domain.AuthorizationGrant) domain.AuthorizationGrant {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.grants[grant.ID]; !exists {
		s.order = append(s.order, grant.ID)
	}
	s.grants[grant.ID] = grant
	if _, ok := s.budgets[grant.ID]; !ok {
		s.budgets[grant.ID] = map[string]int{}
	}
	return grant
}

func (s *AuthorizationService) IssueGrant(params IssueGrantParams) domain.AuthorizationGrant {
	scopes := domain.CopyStrings(params.Scopes)
	sortedScopes := domain.CopyStrings(params.Scopes)
	sort.Strings(sortedScopes)
	sortedSubjects := domain.CopyStrings(params.SubjectIDs)
	sort.Strings(sortedSubjects)
	budget := map[string]int{}
	for key, value := range params.Budget {
		budget[key] = value
	}
	grant := domain.AuthorizationGrant{
		ID: domain.MintID("grn", map[string]any{
			"actor":    params.Actor,
			"scopes":   sortedScopes,
			"subjects": sortedSubjects,
			"issuedAt": params.IssuedAt,
		}),
		Actor:         params.Actor,
		Scopes:        scopes,
		SubjectIDs:    domain.CopyStrings(params.SubjectIDs),
		IssuedAt:      params.IssuedAt,
		ExpiresAt:     params.ExpiresAt,
		Budget:        budget,
		MaxInputBytes: params.MaxInputBytes,
	}
	return s.AddGrant(grant)
}

func (s *AuthorizationService) Grant(id string) (domain.AuthorizationGrant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[id]
	return grant, ok
}

func (s *AuthorizationService) GrantsFor(actor domain.ActorRef) []domain.AuthorizationGrant {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.AuthorizationGrant{}
	for _, id := range s.order {
		if grant := s.grants[id]; grant.Actor.ID == actor.ID {
			out = append(out, grant)
		}
	}
	return out
}

// Evaluate is pure evaluation. Every denial names the rule that produced it.
// The request itself is schema-validated first: a malformed request never
// reaches an authorization decision.
func (s *AuthorizationService) Evaluate(params EvaluateParams) AuthorizationDecision {
	request, contract := params.Request, params.Contract
	base := AuthorizationDecision{
		CapabilityID:      request.CapabilityID,
		CapabilityVersion: request.CapabilityVersion,
		Actor:             request.RequestedBy,
		SubjectID:         request.SubjectID,
		RequiredScopes:    domain.CopyStrings(contract.RequiredScopes),
		DecidedAt:         params.DecidedAt,
		PolicyRef:         AuthzPolicyRef + "@" + AuthzPolicyVersion,
		ID: domain.MintID("grn", map[string]any{
			"kind":         "decision",
			"capabilityId": request.CapabilityID,
			"actor":        request.RequestedBy.ID,
			"subjectId":    request.SubjectID,
			"sessionRef":   request.SessionRef,
			"decidedAt":    params.DecidedAt,
		}),
	}
	deny := func(reason string, reasons []string) AuthorizationDecision {
		d := base
		d.Outcome = "denied"
		d.GrantedScopes = []string{}
		d.Reasons = reasons
		d.DenialReason = reason
		return d
	}

	if issues := domain.ValidateCapabilityRequest(request); len(issues) > 0 {
		reasons := make([]string, len(issues))
		for i, issue := range issues {
			path := issue.Path
			if path == "" {
				path = "request"
			}
			reasons[i] = path + ": " + issue.Message
		}
		return deny(domain.DenialInvalidRequest, reasons)
	}
	if !domain.IsAllowlistedCapability(request.CapabilityID) {
		return deny(domain.DenialNotAllowlisted, []string{fmt.Sprintf("capability %q is not on the allowlist; arbitrary capability invocation is not permitted", request.CapabilityID)})
	}
	if contract.ID != request.CapabilityID {
		return deny(domain.DenialUnknownCapability, []string{fmt.Sprintf("contract id %q does not match the requested capability", contract.ID)})
	}
	if params.InputBytes > contract.MaxInputBytes {
		return deny(domain.DenialInputTooLarge, []string{fmt.Sprintf("canonical input is %d bytes; capability %q allows at most %d", params.InputBytes, contract.ID, contract.MaxInputBytes)})
	}
	// User-bound operations must be requested by, or on behalf of, a user actor.
	kind := request.RequestedBy.Kind
	if kind != domain.ActorUser && kind != domain.ActorCapability && kind != domain.ActorSystem {
		return deny(domain.DenialActorNotUserBound, []string{fmt.Sprintf("actor kind %q may not request capabilities on a user's behalf", kind)})
	}

	var candidates []domain.AuthorizationGrant
	if request.AuthorizationRef == "" {
		candidates = s.GrantsFor(request.RequestedBy)
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	} else if grant, ok := s.Grant(request.AuthorizationRef); ok {
		candidates = []domain.AuthorizationGrant{grant}
	}
	if len(candidates) == 0 {
		return deny(domain.DenialNoAuthorization, []string{fmt.Sprintf("no authorization grant exists for actor %q to invoke %q", request.RequestedBy.ID, request.CapabilityID)})
	}

	// Multiple grants may apply to one actor. A request is authorized when any
	// grant covers it; otherwise the most informative denial is reported.
	decisions := make([]AuthorizationDecision, len(candidates))
	for i, grant := range candidates {
		decisions[i] = s.evaluateAgainstGrant(grant, params, base)
		if decisions[i].Outcome == "granted" {
			return decisions[i]
		}
	}
	sort.SliceStable(decisions, func(i, j int) bool {
		ri, rj := denialRank(decisions[i]), denialRank(decisions[j])
		if ri != rj {
			return ri < rj
		}
		return decisions[i].ID < decisions[j].ID
	})
	return decisions[0]
}

func (s *AuthorizationService) evaluateAgainstGrant(grant domain.AuthorizationGrant, params EvaluateParams, base AuthorizationDecision) AuthorizationDecision {
	request, contract := params.Request, params.Contract
	deny := func(reason string, reasons []string) AuthorizationDecision {
		d := base
		d.Outcome = "denied"
		d.GrantedScopes = domain.CopyStrings(grant.Scopes)
		d.Reasons = reasons
		d.DenialReason = reason
		d.GrantRef = grant.ID
		return d
	}

	s.mu.Lock()
	used := s.budgets[grant.ID][request.CapabilityID]
	s.mu.Unlock()
	limit := grant.Budget[request.CapabilityID]

	if grant.ExpiresAt != "" && grant.ExpiresAt <= params.DecidedAt {
		return deny(domain.DenialExpiredGrant, []string{fmt.Sprintf("grant %s expired at %s; authorization decisions are not reused past expiry", grant.ID, grant.ExpiresAt)})
	}
	if !domain.Contains(grant.SubjectIDs, request.SubjectID) && !domain.Contains(grant.SubjectIDs, "*") {
		return deny(domain.DenialSubjectNotGranted, []string{fmt.Sprintf("grant %s does not cover subject %q", grant.ID, request.SubjectID)})
	}
	missing := []string{}
	for _, scope := range contract.RequiredScopes {
		if !domain.Contains(grant.Scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) > 0 {
		return deny(domain.DenialMissingScopes, []string{fmt.Sprintf("grant %s is missing required scope(s): %s", grant.ID, strings.Join(missing, ", "))})
	}
	if limit <= 0 {
		return deny(domain.DenialBudgetExceeded, []string{fmt.Sprintf("grant %s has no invocation budget for %q", grant.ID, request.CapabilityID)})
	}
	if used >= limit {
		return deny(domain.DenialBudgetExceeded, []string{fmt.Sprintf("grant %s has consumed its budget for %q (%d/%d)", grant.ID, request.CapabilityID, used, limit)})
	}
	if grant.MaxInputBytes != nil && params.InputBytes > *grant.MaxInputBytes {
		return deny(domain.DenialInputTooLarge, []string{fmt.Sprintf("input is %d bytes; grant %s allows at most %d", params.InputBytes, grant.ID, *grant.MaxInputBytes)})
	}

	remaining := limit - used - 1
	d := base
	d.Outcome = "granted"
	d.GrantedScopes = domain.CopyStrings(grant.Scopes)
	d.Reasons = []string{fmt.Sprintf("grant %s covers subject %s with scope(s) %s", grant.ID, request.SubjectID, strings.Join(contract.RequiredScopes, ", "))}
	d.BudgetRemaining = &remaining
	d.GrantRef = grant.ID
	return d
}

// Consume records consumption for a granted decision.
func (s *AuthorizationService) Consume(decision AuthorizationDecision) {
	if decision.Outcome != "granted" || decision.GrantRef == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.budgets[decision.GrantRef] == nil {
		s.budgets[decision.GrantRef] = map[string]int{}
	}
	s.budgets[decision.GrantRef][decision.CapabilityID]++
}

var denialRanks = map[string]int{
	domain.DenialInvalidRequest:     0,
	domain.DenialInputTooLarge:      1,
	domain.DenialBudgetExceeded:     2,
	domain.DenialMissingScopes:      3,
	domain.DenialSubjectNotGranted:  4,
	domain.DenialExpiredGrant:       5,
	domain.DenialActorNotUserBound:  6,
	domain.DenialUnknownCapability:  7,
	domain.DenialNotAllowlisted:     8,
	domain.DenialNoAuthorization:    9,
	domain.DenialInvariantViolation: 10,
}

func denialRank(decision AuthorizationDecision) int {
	if decision.DenialReason == "" {
		return 99
	}
	return denialRanks[decision.DenialReason]
}

func AuthorizationRejections(decision AuthorizationDecision) []domain.Rejection {
	reason := decision.DenialReason
	if reason == "" {
		reason = "denied"
	}
	code := "AUTHZ." + strings.ToUpper(reason)
	out := make([]domain.Rejection, len(decision.Reasons))
	for i, message := range decision.Reasons {
		out[i] = domain.NewRejection(code, message, "authorization")
	}
	return out
}

func ContractFor(capabilityID string) (domain.CapabilityContract, bool) {
	return domain.LookupCapabilityContract(capabilityID)
}

// CapabilityScopesAll is the scope set the runtime may request. Kept separate
// so that the default development grant and the domain's capability
// declarations cannot drift apart unnoticed.
func CapabilityScopesAll() []string {
	return domain.CopyStrings(domain.CapabilityScopes)
}
