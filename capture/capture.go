package capture

import (
	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"
	"code.google.com/p/gopacket/pcap"
	"errors"
	"github.com/ipsecdiagtool/ipsecdiagtool/config"
	"log"
)

var ipSecChannel chan gopacket.Packet
var icmpChannel chan gopacket.Packet

//Start creates a new goroutine that captures data from device "ANY".
//It is blocking until the capture-goroutine is ready. Start returns a quit-channel
//that can be used to gracefully shutdown it's capture-goroutine.
func Start(c config.Config, icmpPackets chan gopacket.Packet, ipsecESP chan gopacket.Packet) chan bool {
	initChannels(icmpPackets, ipsecESP)
	quit := make(chan bool)
	captureReady := make(chan bool)
	go capture(c.PcapSnapLen, quit, captureReady, c.PcapFile)
	<-captureReady
	if c.Debug {
		log.Println("Capture Goroutine Ready")
	}
	return quit
}

//initChannels is needed to initialize this package in the tests
func initChannels(icmpPackets chan gopacket.Packet, ipsecESP chan gopacket.Packet) {
	ipSecChannel = ipsecESP
	icmpChannel = icmpPackets
}

//startCapture captures all packets on any interface for an unlimited duration.
//Packets can be filtered by a BPF filter string. (E.g. tcp port 22)
func capture(snaplen int32, quit chan bool, captureReady chan bool, pcapFile string) error {
	log.Println("Waiting for MTU-Analyzer packet")
	var handle *pcap.Handle
	var err error
	if pcapFile != "" {
		log.Println("Reading packet loss data from pcap-file:", pcapFile)
		handle, err = pcap.OpenOffline(pcapFile)
	} else {
		handle, err = pcap.OpenLive("any", 65000, true, 100)
	}

	if err != nil {
		return err
	} else {
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		captureReady <- true

		for {
			select {
			case packet := <-packetSource.Packets():
				if packet != nil {
					//TODO: remove
					log.Println("handling packet")
					if packet.Layer(layers.LayerTypeIPSecESP) != nil {
						log.Println("detected ipsec packet")
						putChannel(packet, ipSecChannel)
					}
					if packet.Layer(layers.LayerTypeICMPv4) != nil {
						putChannel(packet, icmpChannel)
					}
				}
			case <-quit:
				log.Println("Received quit message, stopping Listener.")
				return nil
			}
		}
	}
}

func putChannel(packet gopacket.Packet, channel chan gopacket.Packet) error {
	select {
	// Put packets in channel unless full
	case channel <- packet:
	default:
		msg := "Channel full, discarding packet."
		return errors.New(msg)
		if config.Debug {
			log.Println(msg)
		}
	}
	return nil
}
