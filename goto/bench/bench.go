package main

import (
	"flag"
	"fmt"
	"github.com/nf/stat"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	n          = flag.Int("n", 10, "")
	host       = flag.String("host", "localhost:8080", "")
	statServer = flag.String("stats", "localhost:8090", "")
	hosts      []string
	hostRe     = regexp.MustCompile("http://[a-zA-Z0-9:.]+")
)

const (
	fooUrl    = "http://github.com/kedebug"
	monDelay  = 1e9
	getDelay  = 100e6
	getters   = 10
	postDelay = 100e6
	posters   = 1
)

var (
	newUrl  = make(chan string)
	randUrl = make(chan string)
)

func keeper() {
	var urls []string
	urls = append(urls, <-newUrl)
	for {
		r := urls[rand.Intn(len(urls))]
		select {
		case u := <-newUrl:
			for _, h := range hosts {
				u = hostRe.ReplaceAllString(u, "http://"+h)
				urls = append(urls, u)
			}
		case randUrl <- r:
		}
	}
}

func post() {
	u := fmt.Sprintf("http://%s/add", hosts[rand.Intn(len(hosts))])
	r, err := http.PostForm(u, url.Values{"url": {fooUrl}})
	if err != nil {
		log.Println("post:", err)
		return
	}
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("post:", err)
		return
	}
	newUrl <- string(b)
	stat.In <- "put"
}

func get() {
	u := <-randUrl
	req, err := http.NewRequest("HEAD", u, nil)
	r, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		log.Println("get:", err)
		return
	}
	defer r.Body.Close()
	_, err = ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("get:", err)
		return
	}
	if l := r.Header.Get("Location"); l != fooUrl {
		log.Println("get: wrong Location", l)
	}
	stat.In <- "get"
}

func loop(fn func(), delay time.Duration) {
	for {
		fn()
		time.Sleep(delay)
	}
}

func main() {
	flag.Parse()
	hosts = strings.Split(*host, ",")
	rand.Seed(time.Now().UnixNano())
	go keeper()
	for i := 0; i < getters*(*n); i++ {
		go loop(get, getDelay)
	}
	for i := 0; i < posters*(*n); i++ {
		go loop(post, postDelay)
	}
	stat.Process = "!bench"
	stat.Monitor(*statServer)
}
