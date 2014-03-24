package tribimpl

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/libstore"
	"github.com/kedebug/golang-programming/15-440/P2-F11/lsplog"
	tp "github.com/kedebug/golang-programming/15-440/P2-F11/tribproto"
	"sort"
	"strconv"
	"sync/atomic"
	"time"
)

type Tribserver struct {
	store    *libstore.Libstore
	hostport string
	id       int32
}

func NewTribserver(master, myhostport string) *Tribserver {
	lsplog.SetVerbose(3)
	lsplog.Vlogf(1, "[Tribserver] new server, master:%s, local:%s\n", master, myhostport)

	srv := new(Tribserver)
	store, err := libstore.NewLibstore(master, myhostport, libstore.NONE)
	if lsplog.CheckReport(1, err) {
		return nil
	}

	atomic.StoreInt32(&srv.id, 0)
	srv.store = store
	srv.hostport = myhostport

	return srv
}

func (ts *Tribserver) CreateUser(
	args *tp.CreateUserArgs, reply *tp.CreateUserReply) error {

	user_key := fmt.Sprintf("%s:U", args.Userid)
	// trib_key := fmt.Sprintf("%s:T", args.Userid)
	// foll_key := fmt.Sprintf("%s:F", args.Userid)

	if _, err := ts.store.Get(user_key); err == nil {
		reply.Status = tp.EEXISTS
		return nil
	}

	if err := ts.store.Put(user_key, args.Userid); err != nil {
		lsplog.Vlogf(2, "[Tribserver] user exists: %v\n", err)
		reply.Status = tp.EEXISTS
		return nil
	}

	// if err := ts.store.Put(trib_key, ""); lsplog.CheckReport(0, err) {
	// 	reply.Status = tp.EEXISTS
	// 	return errors.New("user exists, but put tribble key failed")
	// }

	// if err := ts.store.Put(foll_key, ""); lsplog.CheckReport(0, err) {
	// 	reply.Status = tp.EEXISTS
	// 	return errors.New("user exists, but put follow key failed")
	// }

	reply.Status = tp.OK
	return nil
}

func (ts *Tribserver) AddSubscription(
	args *tp.SubscriptionArgs, reply *tp.SubscriptionReply) error {

	user_key := fmt.Sprintf("%s:U", args.Userid)
	targ_key := fmt.Sprintf("%s:U", args.Targetuser)
	foll_key := fmt.Sprintf("%s:F", args.Userid)

	if _, err := ts.store.Get(user_key); err != nil {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	if _, err := ts.store.Get(targ_key); err != nil {
		reply.Status = tp.ENOSUCHTARGETUSER
		return nil
	}

	if err := ts.store.AppendToList(foll_key, args.Targetuser); err != nil {
		reply.Status = tp.EEXISTS
		return nil
	}

	reply.Status = tp.OK
	return nil
}

func (ts *Tribserver) RemoveSubscription(
	args *tp.SubscriptionArgs, reply *tp.SubscriptionReply) error {

	user_key := fmt.Sprintf("%s:U", args.Userid)
	targ_key := fmt.Sprintf("%s:U", args.Targetuser)
	foll_key := fmt.Sprintf("%s:F", args.Userid)

	if _, err := ts.store.Get(user_key); err != nil {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	if _, err := ts.store.Get(targ_key); err != nil {
		reply.Status = tp.ENOSUCHTARGETUSER
		return nil
	}

	if err := ts.store.RemoveFromList(foll_key, args.Targetuser); err != nil {
		reply.Status = tp.ENOSUCHTARGETUSER
		return nil
	}

	reply.Status = tp.OK
	return nil
}

func (ts *Tribserver) GetSubscriptions(
	args *tp.GetSubscriptionsArgs, reply *tp.GetSubscriptionsReply) error {

	user_key := fmt.Sprintf("%s:U", args.Userid)
	foll_key := fmt.Sprintf("%s:F", args.Userid)

	if _, err := ts.store.Get(user_key); err != nil {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	ids, err := ts.store.GetList(foll_key)
	if err != nil {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	reply.Userids = ids
	reply.Status = tp.OK
	return nil
}

// Get posted tribbles
func (ts *Tribserver) GetTribbles(
	args *tp.GetTribblesArgs, reply *tp.GetTribblesReply) error {

	user_key := fmt.Sprintf("%s:U", args.Userid)
	trib_key := fmt.Sprintf("%s:T", args.Userid)

	if _, err := ts.store.Get(user_key); err != nil {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	var ids []string
	var err error

	if ids, err = ts.store.GetList(trib_key); err != nil {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	length := len(ids)
	if length > 100 {
		length = 100
	}
	reply.Tribbles = make([]tp.Tribble, length)

	for i := 0; i < length; i++ {
		key := fmt.Sprintf("%s:%s:%s", args.Userid, ids[len(ids)-1-i], ts.hostport)
		result, err := ts.store.Get(key)
		if err != nil {
			reply.Status = tp.ENOSUCHUSER
			return errors.New("get tribble message error")
		}
		json.Unmarshal([]byte(result), &(reply.Tribbles[i]))
	}
	reply.Status = tp.OK
	return nil

}

func (ts *Tribserver) PostTribble(
	args *tp.PostTribbleArgs, reply *tp.PostTribbleReply) error {

	user_key := fmt.Sprintf("%s:U", args.Userid)
	trib_key := fmt.Sprintf("%s:T", args.Userid)

	if _, err := ts.store.Get(user_key); err != nil {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	id := atomic.AddInt32(&ts.id, 1)

	if err := ts.store.AppendToList(trib_key, strconv.Itoa(int(id))); err != nil {
		reply.Status = tp.EEXISTS
		return nil
	}

	var trib tp.Tribble

	trib.Userid = args.Userid
	trib.Contents = args.Contents
	trib.Posted = time.Now()

	key := fmt.Sprintf("%s:%s:%s", args.Userid, strconv.Itoa(int(id)), ts.hostport)
	val, _ := json.Marshal(trib)

	if err := ts.store.Put(key, string(val)); err != nil {
		reply.Status = tp.EEXISTS
		return nil
	}

	reply.Status = tp.OK
	return nil
}

type Tribs []tp.Tribble

func (t Tribs) Len() int           { return len(t) }
func (t Tribs) Less(i, j int) bool { return t[i].Posted.After(t[j].Posted) }
func (t Tribs) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

// Collect all bribbles from all users followed
func (ts *Tribserver) GetTribblesBySubscription(
	args *tp.GetTribblesArgs, reply *tp.GetTribblesReply) error {

	user_key := fmt.Sprintf("%s:U", args.Userid)
	foll_key := fmt.Sprintf("%s:F", args.Userid)

	if _, err := ts.store.Get(user_key); lsplog.CheckReport(2, err) {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	follows, err := ts.store.GetList(foll_key)
	if lsplog.CheckReport(3, err) {
		reply.Status = tp.ENOSUCHUSER
		return nil
	}

	for i := 0; i < len(follows); i++ {
		var getargs tp.GetTribblesArgs
		var getreply tp.GetTribblesReply

		getargs.Userid = follows[i]
		err = ts.GetTribbles(&getargs, &getreply)
		if lsplog.CheckReport(1, err) {
			reply.Status = tp.ENOSUCHUSER
			return err
		}
		reply.Tribbles = append(reply.Tribbles, getreply.Tribbles...)
	}

	var tribs Tribs = reply.Tribbles
	sort.Sort(tribs)
	if len(tribs) > 100 {
		reply.Tribbles = tribs[:100]
	}
	reply.Status = tp.OK
	return nil
}
