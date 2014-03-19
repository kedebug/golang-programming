package libstore

const (
	NONE
	ALWAYS_LEASE = iota
)

// NewLibstore creates a new instance of the libstore client (*Libstore),
// telling it to contact _server_ as the master storage server.
// Myhostport is the lease revocation callback port.
//  - If "", the client will never request leases;
//  - If set to a non-zero value (e.g., "localhost:port"), the caller
//    must have an HTTP listener and RPC listener reachable at that port.
// Flags is one of the debugging mode flags from above.
func NewLibstore(master, myhostport string, flags int) (*Libstore, error) {
}

func NewLibstore(server, myhostport string, flags int) (*Libstore, error) {
}

func (ls *Libstore) Get(key string) (string, error) {
}

func (ls *Libstore) Put(key, value string) error {
}

func (ls *Libstore) GetList(key string) ([]string, error) {
}

func (ls *Libstore) RemoveFromList(key, removeitem string) error {
}

func (ls *Libstore) AppendToList(key, newitem string) error {
}
