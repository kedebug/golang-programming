package tribclient

import (
	"github.com/kedebug/golang-programming/15-440/P2-F11/tribproto"
	"log"
	"net"
	"net/rpc"
)

// This is official code provided as part of the 15-440 project handout.
// Intended both to see how the RPC system works and to test your
// tribble server.

// Tiny bit of manual stub creation
type Tribbleclient struct {
	serverAddress string
	serverPort    string
	client        *rpc.Client
}

func NewTribbleclient(serverAddress string, serverPort string) (*Tribbleclient, error) {
	client, err := rpc.DialHTTP("tcp", net.JoinHostPort(serverAddress, serverPort))
	if err != nil {
		log.Fatal("Could not connect to server:", err)
	}
	return &Tribbleclient{serverAddress, serverPort, client}, nil
}

func (tc *Tribbleclient) Close() {
	tc.client.Close()
}

func (tc *Tribbleclient) CreateUser(Userid string) (int, error) {
	args := &tribproto.CreateUserArgs{Userid}
	var reply tribproto.CreateUserReply
	err := tc.client.Call("Tribserver.CreateUser", args, &reply)
	if err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *Tribbleclient) GetSubscriptions(Userid string) ([]string, int, error) {
	args := &tribproto.GetSubscriptionsArgs{Userid}
	var reply tribproto.GetSubscriptionsReply
	err := tc.client.Call("Tribserver.GetSubscriptions", args, &reply)
	if err != nil {
		return nil, 0, err
	}
	return reply.Userids, reply.Status, nil
}

func (tc *Tribbleclient) dosub(funcname, Userid, Targetuser string) (int, error) {
	args := &tribproto.SubscriptionArgs{Userid, Targetuser}
	var reply tribproto.SubscriptionReply
	err := tc.client.Call(funcname, args, &reply)
	if err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *Tribbleclient) AddSubscription(Userid, Targetuser string) (int, error) {
	return tc.dosub("Tribserver.AddSubscription", Userid, Targetuser)
}

func (tc *Tribbleclient) RemoveSubscription(Userid, Targetuser string) (int, error) {
	return tc.dosub("Tribserver.RemoveSubscription", Userid, Targetuser)
}

func (tc *Tribbleclient) GetTribbles(Userid string) ([]tribproto.Tribble, int, error) {
	return tc.dotrib("Tribserver.GetTribbles", Userid)
}

func (tc *Tribbleclient) GetTribblesBySubscription(Userid string) ([]tribproto.Tribble, int, error) {
	return tc.dotrib("Tribserver.GetTribblesBySubscription", Userid)
}

func (tc *Tribbleclient) dotrib(funcname, Userid string) ([]tribproto.Tribble, int, error) {
	args := &tribproto.GetTribblesArgs{Userid}
	var reply tribproto.GetTribblesReply
	err := tc.client.Call(funcname, args, &reply)
	if err != nil {
		return nil, 0, err
	}
	return reply.Tribbles, reply.Status, nil
}

func (tc *Tribbleclient) PostTribble(Userid, Contents string) (int, error) {
	args := &tribproto.PostTribbleArgs{Userid, Contents}
	var reply tribproto.PostTribbleReply
	err := tc.client.Call("Tribserver.PostTribble", args, &reply)
	if err != nil {
		return 0, err
	}
	return reply.Status, nil
}
