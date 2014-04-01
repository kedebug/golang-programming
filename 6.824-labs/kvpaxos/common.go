package kvpaxos

const (
	OK       = "OK"
	ErrNoKey = "ErrNoKey"
)

type Err string

type PutArgs struct {
	// You'll have to add definitions here.
	Key   string
	Value string
}

type PutReply struct {
	Err Err
}

type GetArgs struct {
	// You'll have to add definitions here.
	Key string
}

type GetReply struct {
	Err   Err
	Value string
}
