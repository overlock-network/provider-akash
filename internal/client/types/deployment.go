package types

type DeploymentId struct {
	Dseq  string `json:"dseq"`
	Owner string `json:"owner"`
}

type DeploymentInfo struct {
	State        string       `json:"state"`
	DeploymentId DeploymentId `json:"deployment_id"`
	Hash         string       `json:"hash,omitempty"`       // SDL hash from deployment
	CreatedAt    int64        `json:"created_at,omitempty"` // Block height when deployment was created
}

type EscrowAccountBalance struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

type EscrowAccount struct {
	Owner   string               `json:"owner"`
	State   string               `json:"state"`
	Balance EscrowAccountBalance `json:"balance"`
}

type Deployment struct {
	DeploymentInfo DeploymentInfo `json:"deployment"`
	EscrowAccount  EscrowAccount  `json:"escrow_account"`
}

type DeploymentResponse struct {
	Deployments []Deployment `json:"deployments"`
}

type DeploymentCreateRequest struct {
	SDL      string `json:"sdl"`
	Deposit  int64  `json:"deposit"`
	Currency string `json:"currency"`
}
