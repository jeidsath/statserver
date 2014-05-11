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

var dataStore map[string]*Application

type EventsJson struct {
	Count   uint64   `json:"count"`
	GoodIps []string `json:"good_ips"`
	BadIps  []string `json:"bad_ips"`
}

type Application struct {
	Sha string
	Ips map[int64]uint64
}

func (app *Application) addIp(ip int64) {
	if app.Ips == nil {
		app.Ips = make(map[int64]uint64)
	}
	app.Ips[ip] += 1
}

func reduceIp(ip int64) int64 {
	return (ip / 16) * 16
}

func stringIp(ip int64) string {
	ipSplit := [4]int{}
	for ii := 0; ii < 4; ii++ {
		shift := uint((3 - ii) * 8)
		ipSplit[ii] = int((ip & (255 << shift)) >> shift)
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

func jsonForApp(sha string) (string, error) {

	ll, ok := dataStore[sha]

	if !ok {
		return "", errors.New("Not found")
	}

	// Looks like we're just dealing with IPv4
	mm := make(map[int64]uint64)

	ej := EventsJson{}
	for k, v := range ll.Ips {
		ej.Count += v
		mm[reduceIp(k)] += v
	}

	maxCount := uint64(0)
	maxIp := int64(0)
	for k, v := range mm {
		if v > maxCount {
			maxIp = k
			maxCount = v
		}
	}

	goodIpMap := make(map[int64]int)
	badIpMap := make(map[int64]int)

	for k, _ := range ll.Ips {
		ipNum := reduceIp(k)
		if ipNum == maxIp {
			goodIpMap[k] = 1
		} else {
			badIpMap[k] = 1
		}
	}

	ej.GoodIps = []string{}
	ej.BadIps = []string{}

	for k, _ := range goodIpMap {
		ej.GoodIps = append(ej.GoodIps, stringIp(k))
	}

	for k, _ := range badIpMap {
		ej.BadIps = append(ej.BadIps, stringIp(k))
	}

	sort.Strings(ej.GoodIps)
	sort.Strings(ej.BadIps)

	out, err := json.Marshal(ej)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func appData(w http.ResponseWriter, r *http.Request) {
	pathChunks := strings.Split(r.URL.Path, "/")
	if len(pathChunks) != 3 && pathChunks[1] != "image" ||
		len(pathChunks[2]) != 64 {
		http.NotFound(w, r)
		return
	}
	out, err := jsonForApp(pathChunks[2])
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, string(out))
}

func storeShaIp(sha string, ip int64) {
	if dataStore == nil {
		dataStore = make(map[string]*Application)
	}
	_, ok := dataStore[sha]
	if !ok {
		dataStore[sha] = &Application{sha, nil}
	}
	dataStore[sha].addIp(ip)
}

func udpListen(addr string) {
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

		storeShaIp(*ipEvent.AppSha256, *ipEvent.Ip)
	}
}

func tcpListen(addr string, cc chan bool) {
	http.HandleFunc("/events/", func(w http.ResponseWriter, r *http.Request) {
		appData(w, r)
	})
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		dataStore = nil
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
	go udpListen(":3001")
	cc := make(chan bool)
	go tcpListen(":3000", cc)
	waitForQuitSig(cc)
}
