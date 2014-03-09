package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Item struct {
	Title, Channel, GUID string
}

type Fetcher interface {
	Fetch() (items []Item, next time.Time, err error)
}

type Subscription interface {
	Updates() <-chan Item
	Close() error
}

func Subscribe(fetcher Fetcher) Subscription {
	s := &sub{
		fetcher: fetcher,
		updates: make(chan Item),
		closing: make(chan chan error),
	}
	go s.loop()
	return s
}

type sub struct {
	fetcher Fetcher
	updates chan Item
	closing chan chan error
}

func (s *sub) Updates() <-chan Item {
	return s.updates
}

func (s *sub) Close() error {
	errc := make(chan error)
	s.closing <- errc
	return <-errc
}

func (s *sub) loop() {
	const maxPending = 10
	type fetchResult struct {
		fetched []Item
		next    time.Time
		err     error
	}
	var fetchDone chan fetchResult
	var pending []Item
	var next time.Time
	var err error
	var seen = make(map[string]bool)

	for {
		var fetchDelay time.Duration
		if now := time.Now(); next.After(now) {
			fetchDelay = next.Sub(now)
		}
		var startFetch <-chan time.Time
		if fetchDone == nil && len(pending) < maxPending {
			startFetch = time.After(fetchDelay)
		}

		var first Item
		var updates chan Item
		if len(pending) > 0 {
			first = pending[0]
			updates = s.updates
		}

		select {
		case <-startFetch:
			fetchDone = make(chan fetchResult, 1)
			go func() {
				fetched, next, err := s.fetcher.Fetch()
				fetchDone <- fetchResult{fetched, next, err}

			}()
		case result := <-fetchDone:
			fetchDone = nil
			fetched := result.fetched
			next, err = result.next, result.err
			if err != nil {
				next = time.Now().Add(10 * time.Second)
				break
			}
			for _, item := range fetched {
				if !seen[item.GUID] {
					pending = append(pending, item)
					seen[item.GUID] = true
				}
			}
		case errc := <-s.closing:
			errc <- err
			close(s.updates)
			return
		case updates <- first:
			pending = pending[1:]
		}
	}
}

type merge struct {
	subs    []Subscription
	updates chan Item
	quit    chan struct{}
	errs    chan error
}

func Merge(subs ...Subscription) Subscription {
	m := &merge{
		subs:    subs,
		updates: make(chan Item),
		quit:    make(chan struct{}),
		errs:    make(chan error),
	}
	for _, sub := range subs {
		go func(s Subscription) {
			for {
				var item Item
				select {
				case item = <-s.Updates():
				case <-m.quit:
					m.errs <- s.Close()
					return
				}
				select {
				case m.updates <- item:
				case <-m.quit:
					m.errs <- s.Close()
					return
				}
			}
		}(sub)
	}
	return m
}

func (m *merge) Updates() <-chan Item {
	return m.updates
}

func (m *merge) Close() (err error) {
	close(m.quit)
	for _ = range m.subs {
		if e := <-m.errs; e != nil {
			err = e
		}
	}
	close(m.updates)
	return
}

type fetcher struct {
	channel string
	items   []Item
}

func Fetch(domain string) Fetcher {
	return &fetcher{
		channel: domain,
	}
}

func (f *fetcher) Fetch() (items []Item, next time.Time, err error) {
	now := time.Now()
	next = now.Add(time.Duration(rand.Intn(5)) * 500 * time.Millisecond)
	item := Item{
		Channel: f.channel,
		Title:   fmt.Sprintf("Item %d", len(f.items)),
	}
	item.GUID = item.Channel + "/" + item.Title
	f.items = append(f.items, item)
	items = []Item{item}
	return
}

func main() {
	rand.Seed(time.Now().UnixNano())

	merged := Merge(
		Subscribe(Fetch("blog.golang.org")),
		Subscribe(Fetch("github.com/kedebug")),
		Subscribe(Fetch("cnblogs.com/kedebug")))

	time.AfterFunc(3*time.Second, func() {
		fmt.Println("closed:", merged.Close())
	})

	for item := range merged.Updates() {
		fmt.Println(item.Channel, item.Title)
	}
}
