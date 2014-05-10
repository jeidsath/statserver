package main

import (
        "errors"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

var dataStore map[string]*Application

type EventsJson struct {
	Count   uint64   `json:"count"`
	GoodIps []string `json:"good_ips"`
	BadIps  []string `json:"bad_ips"`
}

type Application struct {
	Sha string
	Ips map[string]uint64
}

func (app *Application) addIp(ip []byte) {
	if app.Ips == nil {
		app.Ips = make(map[string]uint64)
	}
	app.Ips[stringIp(ip)] += 1
}

func reduceIp(ip string) int {
	ipSplit := strings.Split(ip, ".")
	out := 0
	for ii := 0; ii < 3; ii++ {
		subNum, _ := strconv.Atoi(ipSplit[ii])
		out += int(math.Pow(256, float64(3-ii))) * subNum
	}
	subNum, _ := strconv.Atoi(ipSplit[3])
	out += subNum / 16
	return out
}

func stringIp(ip []byte) string {
	ipStr := ""
	for ii := 3; ii >= 0; ii-- {
		ipStr += strconv.Itoa(int(ip[ii]))
		if ii != 0 {
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
	mm := make(map[int]uint64)

	ej := EventsJson{}
	for k, v := range ll.Ips {
		ej.Count += v
		mm[reduceIp(k)] += v
	}

	maxCount := uint64(0)
	maxIp := 0
	for k, v := range mm {
		if v > maxCount {
			maxIp = k
			maxCount = v
		}
	}

	goodIpMap := make(map[string]int)
	badIpMap := make(map[string]int)

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
		ej.GoodIps = append(ej.GoodIps, k)
	}

	for k, _ := range badIpMap {
		ej.BadIps = append(ej.BadIps, k)
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

func storeShaIp(sha string, ip []byte) {
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
		if err != nil || n != 72 {
			fmt.Println(err)
			continue
		}
		if n != 72 {
			fmt.Println("ERROR: bad packet")
			continue
		}

		storeShaIp(string(buf[0:64]), buf[64:72])
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
