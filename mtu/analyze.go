package mtu

import (
	"code.google.com/p/gopacket/examples/util"
	"log"
	"net"
	"strconv"
	"time"
	"github.com/ipsecdiagtool/ipsecdiagtool/config"
)

//Package internal temp. variables
var startingMTU int
var mtuOKchan (chan int)
var conf config.Config

//Analyze determines the ideal MTU (Maximum Transmission Unit) between two nodes
//by sending increasingly big packets between them. Analyze determine the MTU
//by running FindMTU multiple times. However it is not a daemon. Once it has found
//the ideal MTU it reports it and then closes shop.
func Analyze(c config.Config) {
	defer util.Run()()
	log.Println("Analyzing MTU..")

	//Setup a channel for communication with capture
	mtuOKchan = make(chan int) // Allocate a channel.
	startingMTU = 500
	conf = c

	//Capture all traffic via goroutine in separate thread
	go startCapture("tcp port " + strconv.Itoa(conf.Port))

	//TODO: may not belong into production code.
	//Slow down if we're running tests on localhost
	if(conf.DestinationIP == "127.0.0.1"){
		time.Sleep(1000 * time.Millisecond)
	}

	//MTU detection algorithm
	var detectedMTU = startingMTU
	var incStep = conf.IncrementationStep
	for i := 0; i < config.MTUIterations; i++ {
		detectedMTU = FindMTU(
			net.ParseIP(conf.SourceIP),
			net.ParseIP(conf.DestinationIP),
			conf.Port,
			detectedMTU,
			incStep)
		incStep = incStep/2
	}
	log.Println("MTU found:", detectedMTU)
}

//FindMTU discovers the MTU between two nodes and returns it as an int value. FindMTU currently
//increases the packet size until it runs into a timeout. Once it runs into the timeout it returns
//the last known as good MTU.
func FindMTU(srcIP net.IP, destIP net.IP, destPort int, startMTU int, increment int) int {
	var goodMTU, nextMTU = 0, startMTU

	//1. Initiate MTU discovery by sending first packet.
	sendPacket(srcIP, destIP, destPort, nextMTU, "MTU?")

	//2. Either we get a message from our mtu channel or the timeout channel will message us after 10s.
	for {
		//2.1 Setting up the timeout channel
		//http://blog.golang.org/go-concurrency-patterns-timing-out-and
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(10 * time.Second)
			timeout <- true
		}()

		select {
		case <-mtuOKchan:
			goodMTU = nextMTU
			nextMTU += increment
			time.Sleep(1000 * time.Millisecond)
			sendPacket(srcIP, destIP, destPort, nextMTU, "MTU?")
		case <-timeout:
			log.Println("Timeout has occured. We've steped over the MTU!")
			log.Println("Last known good MTU:", goodMTU)
			return goodMTU
		}
	}
}
