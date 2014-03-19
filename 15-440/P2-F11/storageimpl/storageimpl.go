package storageimpl

import (
	sp "goproc/15-440/P2-F11/storageproto"
)

type StorageServer struct {
}

func NewStorageserver(master string, numnodes int, port int, nodeid uint32) *StorageServer {
}

func (ss *StorageServer) Get(args *sp.GetArgs, reply *sp.GetReply) error {
}

func (ss *StorageServer) GetList(args *sp.GetArgs, reply *sp.GetListReply) error {
}

func (ss *StorageServer) Put(args *sp.PutArgs, reply *sp.PutReply) error {
}

func (ss *StorageServer) AppendToList(args *sp.PutArgs, reply *sp.PutReply) error {
}

func (ss *StorageServer) RemoveFromList(args *sp.PutArgs, reply *sp.PutReply) error {
}

func (ss *StorageServer) Register(args *sp.RegisterArgs, reply *sp.RegisterReply) error {
}

func (ss *StorageServer) GetServers(args *sp.GetServersArgs, reply *sp.RegisterReply) error {
}
