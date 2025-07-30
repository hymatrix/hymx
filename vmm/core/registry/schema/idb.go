package schema

type IDB interface {
	GetId() (string, error)
	GetTokenPid() (string, error)

	Register(node Node) error
	Unregister(accid string) error
	GetNode(accid string) (*Node, error)
	GetNodes() (map[string]Node, error)

	RegisterProcess(accid, pid string) error
	UnregisterProcess(accid, pid string) error
	GetProcesses(accid string) (procs []string, err error)
	GetNodesByProcess(pid string) (nodes []Node, err error)

	Checkpoint() (data string, err error)
	Restore(data string) error
}
