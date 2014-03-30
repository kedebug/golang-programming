package pbservice

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongServer = "ErrWrongServer"
	OP_GET         = "get"
	OP_PUT         = "put"
)

type Err string

type PutArgs struct {
	Key   string
	Value string
}

type PutReply struct {
	Err Err
}

type GetArgs struct {
	Key string
}

type GetReply struct {
	Err   Err
	Value string
}

type ForwardArgs struct {
	Op    string
	Key   string
	Value string
}

type ForwardReply struct {
	Err   Err
	Value string
}

type SyncArgs struct {
	Store map[string]string
}

type SyncReply struct {
	Err Err
}

// Your RPC definitions here.
