package storageproto

// Status codes
const (
	OK = iota
	EKEYNOTFOUND
	EITEMNOTFOUND // lists
	EWRONGSERVER
	EPUTFAILED
	EITEMEXISTS // lists, duplicate put
)

// Leasing
const (
	QUERY_CACHE_SECONDS = 10 // Time period for tracking queries
	QUERY_CACHE_THRESH  = 3  // if 3 queries in QUERY_CACHE_SECONDS, get lease
	LEASE_SECONDS       = 10 // Leases are valid for LEASE_SECONDS seconds
	LEASE_GUARD_SECONDS = 2  // And the server should wait an extra 2
)

type LeaseStruct struct {
	Granted      bool
	ValidSeconds int
}

type GetArgs struct {
	Key         string
	WantLease   bool
	LeaseClient string // host:port of client that wants lease, for callback
}

type GetReply struct {
	Status int
	Value  string
	Lease  LeaseStruct
}

type GetListReply struct {
	Status int
	Value  []string
	Lease  LeaseStruct
}

type PutArgs struct {
	Key   string
	Value string
}

type PutReply struct {
	Status int
}

type Node struct {
	HostPort string
	NodeID   uint32
}

type RegisterArgs struct {
	ServerInfo Node
}

// RegisterReply is sent in response to both Register and GetServers
type RegisterReply struct {
	Ready   bool
	Servers []Node
}

type GetServersArgs struct {
}

// Used by the Cacher RPC
type RevokeLeaseArgs struct {
	Key string
}

type RevokeLeaseReply struct {
	Status int
}
