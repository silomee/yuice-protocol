// Package finance models Yuice's asset-finance payment workflow.
//
// It validates payment proposals and their governance approvals. It does not
// hold keys, submit transactions, or move funds; those actions require a
// separately reviewed chain adapter and an explicit network and token policy.
package finance

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

type Stage string

const (
	StageConnected  Stage = "connected"
	StageUnderstood Stage = "understood"
	StageGoverned   Stage = "governed"
	StageAuthorized Stage = "authorized"
	StageExecuted   Stage = "executed"
)

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReject  Decision = "reject"
)

// PaymentProposal describes an asset-level payment using integer token base
// units. AmountBaseUnits must never be expressed as a floating-point value.
type PaymentProposal struct {
	ID              string     `json:"id"`
	AssetID         string     `json:"assetId"`
	ChainID         uint64     `json:"chainId"`
	TokenAddress    string     `json:"tokenAddress"`
	Recipient       string     `json:"recipient"`
	AmountBaseUnits string     `json:"amountBaseUnits"`
	Purpose         string     `json:"purpose"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	Stage           Stage      `json:"stage"`
	Approvals       []Approval `json:"approvals,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type Approval struct {
	ApproverID string    `json:"approverId"`
	Role       string    `json:"role"`
	Decision   Decision  `json:"decision"`
	Reason     string    `json:"reason,omitempty"`
	At         time.Time `json:"at"`
}

// Policy has no implicit defaults: callers must supply the chain, token,
// recipient and payment limits for the deployment they are configuring.
type Policy struct {
	AllowedChainIDs       map[uint64]struct{}
	AllowedTokenAddresses map[string]struct{}
	AllowedRecipients     map[string]struct{}
	MaxPaymentBaseUnits   string
	MinApprovals          int
	RequiredApproverRoles map[string]struct{}
}

var (
	addressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	amountPattern  = regexp.MustCompile(`^[0-9]+$`)
	maxUint256     = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
)

func NewProposal(p PaymentProposal) (PaymentProposal, error) {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.AssetID) == "" {
		return PaymentProposal{}, errors.New("proposal ID and asset ID are required")
	}
	if p.ChainID == 0 {
		return PaymentProposal{}, errors.New("chain ID must be explicit")
	}
	if !addressPattern.MatchString(p.TokenAddress) || !addressPattern.MatchString(p.Recipient) {
		return PaymentProposal{}, errors.New("token and recipient must be 20-byte EVM addresses")
	}
	if !validAmount(p.AmountBaseUnits) {
		return PaymentProposal{}, errors.New("amount must be a positive uint256 in token base units")
	}
	if strings.TrimSpace(p.Purpose) == "" || strings.TrimSpace(p.IdempotencyKey) == "" {
		return PaymentProposal{}, errors.New("payment purpose and idempotency key are required")
	}
	if p.Stage != "" && p.Stage != StageConnected {
		return PaymentProposal{}, fmt.Errorf("new proposal must start at %q", StageConnected)
	}
	p.Stage = StageConnected
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	return p, nil
}

// MarkUnderstood records that asset and financial inputs have been reconciled.
func (p PaymentProposal) MarkUnderstood() (PaymentProposal, error) {
	if p.Stage != StageConnected {
		return PaymentProposal{}, fmt.Errorf("cannot reconcile proposal in stage %q", p.Stage)
	}
	p.Stage = StageUnderstood
	return p, nil
}

// Govern validates the configured limits and distinct human approvals.
func (p PaymentProposal) Govern(policy Policy) (PaymentProposal, error) {
	if p.Stage != StageUnderstood {
		return PaymentProposal{}, fmt.Errorf("cannot govern proposal in stage %q", p.Stage)
	}
	if err := policy.validate(p); err != nil {
		return PaymentProposal{}, err
	}
	if policy.MinApprovals < 1 {
		return PaymentProposal{}, errors.New("policy must require at least one approval")
	}
	approvers := make(map[string]struct{})
	roles := make(map[string]struct{})
	for _, approval := range p.Approvals {
		if strings.TrimSpace(approval.ApproverID) == "" || approval.At.IsZero() {
			return PaymentProposal{}, errors.New("each approval needs an approver ID and timestamp")
		}
		if approval.Decision == DecisionReject {
			return PaymentProposal{}, fmt.Errorf("proposal rejected by approver %q", approval.ApproverID)
		}
		if approval.Decision != DecisionApprove {
			return PaymentProposal{}, fmt.Errorf("invalid decision from approver %q", approval.ApproverID)
		}
		if _, exists := approvers[approval.ApproverID]; exists {
			return PaymentProposal{}, fmt.Errorf("duplicate approval from approver %q", approval.ApproverID)
		}
		approvers[approval.ApproverID] = struct{}{}
		roles[approval.Role] = struct{}{}
	}
	if len(approvers) < policy.MinApprovals {
		return PaymentProposal{}, fmt.Errorf("policy requires %d distinct approvals", policy.MinApprovals)
	}
	for role := range policy.RequiredApproverRoles {
		if _, ok := roles[role]; !ok {
			return PaymentProposal{}, fmt.Errorf("policy requires an approval from role %q", role)
		}
	}
	p.Stage = StageAuthorized
	return p, nil
}

func (policy Policy) validate(p PaymentProposal) error {
	if _, ok := policy.AllowedChainIDs[p.ChainID]; !ok {
		return fmt.Errorf("chain ID %d is not allowed", p.ChainID)
	}
	if _, ok := policy.AllowedTokenAddresses[strings.ToLower(p.TokenAddress)]; !ok {
		return errors.New("token address is not allowed")
	}
	if _, ok := policy.AllowedRecipients[strings.ToLower(p.Recipient)]; !ok {
		return errors.New("recipient is not allowed")
	}
	if !validAmount(policy.MaxPaymentBaseUnits) {
		return errors.New("policy maximum must be a positive uint256 in token base units")
	}
	amount, _ := new(big.Int).SetString(p.AmountBaseUnits, 10)
	maximum, _ := new(big.Int).SetString(policy.MaxPaymentBaseUnits, 10)
	if amount.Cmp(maximum) > 0 {
		return errors.New("payment exceeds policy maximum")
	}
	return nil
}

func validAmount(s string) bool {
	if !amountPattern.MatchString(s) {
		return false
	}
	amount, ok := new(big.Int).SetString(s, 10)
	return ok && amount.Sign() > 0 && amount.Cmp(maxUint256) <= 0
}
