package main

import (
	"log"
	"net"
	"time"

	pb "SnakeGame/model/proto"
)

const (
	multicastAddress = "239.192.0.4:9192"
)

func connection(nodeRole *pb.NodeRole) {
	// резолвим multicast-адрес
	multicastUDPAddr, err := net.ResolveUDPAddr("udp4", multicastAddress)
	if err != nil {
		log.Fatalf("Error resolving multicast address: %v", err)
	}

	// создаем сокет для multicast
	multicastConn, err := net.ListenMulticastUDP("udp4", nil, multicastUDPAddr)
	if err != nil {
		log.Fatalf("Error creating multicast socket: %v", err)
	}
	defer multicastConn.Close()

	// создаем сокет для остальных сообщений
	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		log.Fatalf("Error resolving local UDP address: %v", err)
	}
	unicastConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		log.Fatalf("Error creating unicast socket: %v", err)
	}
	defer unicastConn.Close()

	localAddrResolved := unicastConn.LocalAddr().(*net.UDPAddr)
	log.Printf("Unicast UDP listening on %v", localAddrResolved)

	if nodeRole == pb.NodeRole_MASTER.Enum() {
		go startMasterAnnouncement(unicastConn, multicastUDPAddr)
	}

	go handleMulticast(multicastConn)
}

func handleMulticast(multicastConn *net.UDPConn) {

}

func startMasterAnnouncement(unicastConn *net.UDPConn, multicastAddr net.Addr) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		//msg := &pb.GameMessage{
		//	MsgSeq: proto.Int64(time.Now().UnixNano()),
		//	Type: &pb.GameMessage_Announcement{
		//		Announcement: &pb.GameMessage_AnnouncementMsg{
		//			Games: []*pb.GameAnnouncement{announcement},
		//		},
		//	},
		//}

		//data, err := proto.Marshal(msg)
		//if err != nil {
		//	log.Printf("Error marshalling AnnouncementMsg: %v", err)
		//	continue
		//}
		//
		//_, err = unicastConn.WriteTo(data, multicastAddr)
		//if err != nil {
		//	log.Printf("Error sending AnnouncementMsg: %v", err)
		//}
	}

}
