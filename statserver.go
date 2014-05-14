package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"./ip_event"
	"code.google.com/p/goprotobuf/proto"
)

type DataStore struct {
	Data map[string]*Application
}

type EventsJson struct {
	Count   uint64   `json:"count"`
	GoodIps []string `json:"good_ips"`
	BadIps  []string `json:"bad_ips"`
}

type Application struct {
	Sha string
	Ips map[IpAddress]uint64
}

type IpAddress struct {
	Address int64
}

func (store *DataStore) init() {
	store.Data = make(map[string]*Application)
}

func (store *DataStore) reset() {
	store.init()
}

func (store *DataStore) eventJson(sha string) (string, error) {
	ll, ok := store.Data[sha]
	if !ok {
		return "", errors.New("Not found")
	}
	return ll.toJson()
}

func (store *DataStore) serveEvents(w http.ResponseWriter, r *http.Request) {
	pathChunks := strings.Split(r.URL.Path, "/")
	if len(pathChunks) != 3 && pathChunks[1] != "image" ||
		len(pathChunks[2]) != 64 {
		http.NotFound(w, r)
		return
	}
	out, err := store.eventJson(pathChunks[2])
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, string(out))
}

func (store *DataStore) insert(sha string, ip IpAddress) {
	_, ok := store.Data[sha]
	if !ok {
		store.Data[sha] = &Application{sha, nil}
	}
	store.Data[sha].addIp(ip)
}

func (ip *IpAddress) topIPForBlock(size int64) IpAddress {
	return IpAddress{(ip.Address / size) * size}
}

func (ip *IpAddress) toString() string {
	ipSplit := [4]int{}
	for ii := 0; ii < 4; ii++ {
		shift := uint((3 - ii) * 8)
		ipSplit[ii] = int((ip.Address & (255 << shift)) >> shift)
	}
	ipStr := ""
	for ii := 0; ii < 4; ii++ {
		ipStr += strconv.Itoa(ipSplit[ii])
		if ii != 3 {
			ipStr += "."
		}
	}
	return ipStr
}

func (app *Application) addIp(ip IpAddress) {
	if app.Ips == nil {
		app.Ips = make(map[IpAddress]uint64)
	}
	app.Ips[ip] += 1
}

func (app *Application) toJson() (string, error) {

	// Histogram the count of IP Addresses by IP block

	reducedAddresses := make(map[IpAddress]uint64)

	ej := EventsJson{}
	for k, v := range app.Ips {
		ej.Count += v
		reducedAddresses[k.topIPForBlock(16)] += v
	}

	maxFrequency := uint64(0)
	mostFrequentIp := IpAddress{}
	for k, v := range reducedAddresses {
		if v > maxFrequency {
			mostFrequentIp = k
			maxFrequency = v
		}
	}

	// Split our IpAddress by block. Good IPs are those whose block matches
	// the highest frequency block

	goodIpMap := make(map[IpAddress]int)
	badIpMap := make(map[IpAddress]int)

	for k, _ := range app.Ips {
		reducedIp := k.topIPForBlock(16)
		if reducedIp == mostFrequentIp {
			goodIpMap[k] = 1
		} else {
			badIpMap[k] = 1
		}
	}

	ej.GoodIps = []string{}
	ej.BadIps = []string{}

	for k, _ := range goodIpMap {
		ej.GoodIps = append(ej.GoodIps, k.toString())
	}

	for k, _ := range badIpMap {
		ej.BadIps = append(ej.BadIps, k.toString())
	}

	sort.Strings(ej.GoodIps)
	sort.Strings(ej.BadIps)

	out, err := json.Marshal(ej)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func udpListen(addr string, store *DataStore) {
	udpAddr, _ := net.ResolveUDPAddr("udp", addr)

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	buf := make([]byte, 1024)

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if n != 72 {
			fmt.Printf("ERROR: bad packet, %v read\n", n)
			continue
		}

		ipEvent := &lookout_backend_coding_questions_q1.IpEvent{}
		err = proto.Unmarshal(buf[0:72], ipEvent)
		if err != nil {
			fmt.Println(err)
			continue
		}

		store.insert(*ipEvent.AppSha256, IpAddress{*ipEvent.Ip})
	}
}

func tcpListen(addr string, cc chan bool, store *DataStore) {
	http.HandleFunc("/events/", func(w http.ResponseWriter, r *http.Request) {
		store.serveEvents(w, r)
	})
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		store.reset()
	})
	http.HandleFunc("/quit", func(w http.ResponseWriter, r *http.Request) {
		cc <- true
	})
	http.ListenAndServe(addr, nil)
}

func waitForQuitSig(cc chan bool) {
	<-cc
}

func main() {
	store := &DataStore{}
	store.init()
	go udpListen(":3001", store)
	cc := make(chan bool)
	go tcpListen(":3000", cc, store)
	waitForQuitSig(cc)
}
