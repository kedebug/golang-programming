package libstore

import (
	"hash/fnv"
)

const (
	NONE         = 0
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
	return iNewLibstore(master, myhostport, flags)
}

func (ls *Libstore) Get(key string) (string, error) {
	return ls.iGet(key)
}

func (ls *Libstore) Put(key, value string) error {
	return ls.iPut(key, value)
}

func (ls *Libstore) GetList(key string) ([]string, error) {
	return ls.iGetList(key)
}

func (ls *Libstore) RemoveFromList(key, removeitem string) error {
	return ls.iRemoveFromList(key, removeitem)
}

func (ls *Libstore) AppendToList(key, newitem string) error {
	return ls.iAppendToList(key, newitem)
}

func StoreHash(key string) uint32 {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return hasher.Sum32()
}
