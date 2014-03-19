package tribimpl

import (
	"goproc/15-440/P2-F11/lsplog"
	tp "goproc/15-440/P2-F11/tribproto"
)

type Tribserver struct {
	id int
}

func NewTribserver(master, myhostport string) *Tribserver {
}

func (ts *Tribserver) CreateUser(
	args *tp.CreateUserArgs, reply *tp.CreateUserReply) error {

}

func (ts *Tribserver) AddSubscription(
	args *tp.SubscriptionArgs, reply *tp.SubscriptionReply) error {
}

func (ts *Tribserver) RemoveSubscription(
	args *tp.SubscriptionArgs, reply *tp.SubscriptionReply) error {
}

func (ts *Tribserver) GetSubscriptions(
	args *tp.GetSubscriptionsArgs, reply *tp.GetSubscriptionsReply) error {
}

// Get posted tribbles
func (ts *Tribserver) GetTribbles(
	args *tp.GetTribblesArgs, reply *tp.GetTribblesReply) error {
}

// Collect all bribbles from all users followed
func (ts *Tribserver) GetTribblesBySubscription(
	args *tp.GetTribblesArgs, reply *tp.GetTribblesReply) error {
}
