package main

import (
	"flag"
	"os"
	"fmt"
	"net/http"
	"log"
	"encoding/binary"
	"net"
	"time"
	"sync"
	"os/signal"
	"syscall"
)

type ProxyStats struct {
	Events uint32
	TotalEvents uint32
	IP4Cnt uint32
	IP4Unique map[uint32]bool
}

var PROXY_STATS ProxyStats
var PROXY_MUTEX *sync.Mutex

func getIP(req *http.Request)(net.IP){

	userIP := net.ParseIP("0.0.0.0")
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		userIP = net.ParseIP(ip)
		if userIP == nil {
			fmt.Println("userip: %q is not IP:port", req.RemoteAddr)
		}
	}

	// This will only be defined when site is accessed via non-anonymous proxy
	// and takes precedence over RemoteAddr
	// Header.Get is case-insensitive
	xff := req.Header.Get("X-Forwarded-For")
	if xff != "" {
		userIP = net.ParseIP(xff)
		if userIP == nil {
			fmt.Println("xff: %q is not IP:port", xff)
		}
	}
	return userIP
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

func handler(w http.ResponseWriter, r *http.Request) {

	ip32 := ip2int(getIP(r))
	PROXY_MUTEX.Lock()
	PROXY_STATS.Events++
	PROXY_STATS.TotalEvents++
	if _, ok := PROXY_STATS.IP4Unique[ip32]; !ok {
		PROXY_STATS.IP4Cnt++
		PROXY_STATS.IP4Unique[ip32] = true
	}
	PROXY_MUTEX.Unlock()
	authinfo := r.Header.Get("X-AuthInfo")
	w.Write([]byte(fmt.Sprintf("Path: %s\nX-AuthInfo: %s\n", r.URL.Path, authinfo)))
}

func reportStats(delay time.Duration) {
	for {
		time.Sleep(delay)
		msg := fmt.Sprintf( "%d - %d,%d,%d",
			time.Now().Unix(),
			PROXY_STATS.Events,PROXY_STATS.TotalEvents,PROXY_STATS.IP4Cnt)
		fmt.Println(msg)
		PROXY_MUTEX.Lock()
		PROXY_STATS.Events = 0
		PROXY_MUTEX.Unlock()
	}
}

func initProxy() {
	
	PROXY_MUTEX.Lock()
	PROXY_STATS.Events = 0
	PROXY_STATS.TotalEvents = 0
	PROXY_STATS.IP4Cnt = 0
	PROXY_STATS.IP4Unique = make(map[uint32]bool, 0)
	PROXY_MUTEX.Unlock()
}

func main() {

	var err error
	PROXY_STATS = ProxyStats{}
	PROXY_MUTEX = &sync.Mutex{}

	tcpPort := flag.String("p", ":9090", "Default tcp port")
	flag.Parse()
	
	// Setup ability to dynamically init proxy. Will not reload initial cmd flags or environment.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func(){
		for _ = range c {
			initProxy()
		}
	}()

	// Init
	initProxy()

	// Setup reporting of stats to syslog/stdout
	go reportStats(time.Second)

	// Listen on localhost
	http.HandleFunc("/", handler)
	fmt.Printf("Listening on %s\n", *tcpPort)
	err = http.ListenAndServe(*tcpPort, nil) // set listen port                                                                                                              
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

