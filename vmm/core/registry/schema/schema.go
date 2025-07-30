package schema

const (
	RoleMain      = "main"
	RoleCandidate = "candidate"
	RoleFollower  = "follower"
)

type Node struct {
	AccId string `json:"Acc-Id"`
	Name  string `json:"Name"`
	Role  string `json:"Role"`
	Desc  string `json:"Desc"`
	URL   string `json:"URL"`
}
