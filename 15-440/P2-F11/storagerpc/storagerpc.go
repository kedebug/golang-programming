// This is a quick adapter to ensure that only the desired interfaces
// are exported, and to define the storage server interface.
//
// Do not modify this file for 15-440.
//
// Implement your changes in your own contrib/storageimpl code.
//

package storagerpc

import (
	"github.com/kedebug/golang-programming/15-440/P2-F11/storageproto"
)

type StorageInterface interface {
	RegisterServer(*storageproto.RegisterArgs, *storageproto.RegisterReply) error
	GetServers(*storageproto.GetServersArgs, *storageproto.RegisterReply) error
	Get(*storageproto.GetArgs, *storageproto.GetReply) error
	GetList(*storageproto.GetArgs, *storageproto.GetListReply) error
	Put(*storageproto.PutArgs, *storageproto.PutReply) error
	AppendToList(*storageproto.PutArgs, *storageproto.PutReply) error
	RemoveFromList(*storageproto.PutArgs, *storageproto.PutReply) error
}

type StorageRPC struct {
	ss StorageInterface
}

func NewStorageRPC(ss StorageInterface) *StorageRPC {
	return &StorageRPC{ss}
}

func (srpc *StorageRPC) Get(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	return srpc.ss.Get(args, reply)
}

func (srpc *StorageRPC) GetList(args *storageproto.GetArgs, reply *storageproto.GetListReply) error {
	return srpc.ss.GetList(args, reply)
}

func (srpc *StorageRPC) Put(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	return srpc.ss.Put(args, reply)
}

func (srpc *StorageRPC) AppendToList(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	return srpc.ss.AppendToList(args, reply)
}

func (srpc *StorageRPC) RemoveFromList(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	return srpc.ss.RemoveFromList(args, reply)
}

func (srpc *StorageRPC) Register(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {
	return srpc.ss.RegisterServer(args, reply)
}

func (srpc *StorageRPC) GetServers(args *storageproto.GetServersArgs, reply *storageproto.RegisterReply) error {
	return srpc.ss.GetServers(args, reply)
}
