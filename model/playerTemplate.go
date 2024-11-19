package model

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
)

type Player struct {
	state            *pb.GameState
	config           *pb.GameConfig
	multicastAddress string
	multicastConn    *net.UDPConn
	unicastConn      *net.UDPConn
	masterAddr       *net.UDPAddr
	msgSeq           int64
	playerInfo       *pb.GamePlayer
	announcement     *pb.GameMessage_AnnouncementMsg
}

func NewPlayer(multicastConn *net.UDPConn) *Player {
	// создаем сокет для остальных сообщений
	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		log.Fatalf("Error resolving local UDP address: %v", err)
	}
	unicastConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		log.Fatalf("Error creating unicast socket: %v", err)
	}

	return &Player{
		multicastAddress: "239.192.0.4:9192",
		multicastConn:    multicastConn,
		unicastConn:      unicastConn,
		msgSeq:           1,
		playerInfo: &pb.GamePlayer{
			Name:  proto.String("Player"),
			Id:    proto.Int32(0),
			Role:  pb.NodeRole_NORMAL.Enum(),
			Type:  pb.PlayerType_HUMAN.Enum(),
			Score: proto.Int32(0),
		},
	}
}

func (p *Player) Start() {
	p.discoverGames()
	go p.receiveMulticastMessages()
	go p.receiveMessages()
}

func (p *Player) receiveMulticastMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := p.multicastConn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error receiving multicast message: %v", err)
			continue
		}

		var msg pb.GameMessage
		err = proto.Unmarshal(buf[:n], &msg)
		if err != nil {
			log.Printf("Error unmarshaling multicast message: %v", err)
			continue
		}

		p.handleMulticastMessage(&msg, addr)
	}
}

func (p *Player) handleMulticastMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Announcement:
		p.masterAddr = addr
		p.announcement = t.Announcement
		log.Printf("Received AnnouncementMsg from %v via multicast", addr)
		p.sendJoinRequest()
	default:
		log.Printf("Received unknown multicast message from %v", addr)
	}
}

func (p *Player) receiveMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := p.unicastConn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			continue
		}

		var msg pb.GameMessage
		err = proto.Unmarshal(buf[:n], &msg)
		if err != nil {
			log.Printf("Error unmarshalling message: %v", err)
			continue
		}

		p.handleMessage(&msg, addr)
	}
}

func (p *Player) handleMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Ack:
		p.playerInfo.Id = proto.Int32(msg.GetReceiverId())
		log.Printf("Joined game with ID: %d", p.playerInfo.GetId())
	case *pb.GameMessage_Announcement:
		p.masterAddr = addr
		p.announcement = t.Announcement
		log.Printf("Received AnnouncementMsg from %v via unicast", addr)
		p.sendJoinRequest()
	case *pb.GameMessage_State:
		p.state = t.State.GetState()
		log.Printf("Received StateMsg with state_order: %d", p.state.GetStateOrder())
	case *pb.GameMessage_Error:
		log.Printf("Error from server: %s", t.Error.GetErrorMessage())
	case *pb.GameMessage_RoleChange:
		log.Printf("Received RoleChangeMsg")
	default:
		log.Printf("Received unknown message")
	}
}

func (p *Player) discoverGames() {
	discoverMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(p.msgSeq),
		Type: &pb.GameMessage_Discover{
			Discover: &pb.GameMessage_DiscoverMsg{},
		},
	}
	p.msgSeq++

	data, err := proto.Marshal(discoverMsg)
	if err != nil {
		log.Fatalf("Error marshalling discovery message: %v", err)
		return
	}

	multicastAddr, err := net.ResolveUDPAddr("udp", p.multicastAddress)
	if err != nil {
		log.Fatalf("Error resolving multicast address: %v", err)
		return
	}

	_, err = p.unicastConn.WriteToUDP(data, multicastAddr)
	if err != nil {
		log.Fatalf("Error sending discovery message: %v", err)
		return
	}

	log.Printf("Sent DiscoverMsg to %v", multicastAddr)
}

func (p *Player) sendJoinRequest() {
	joinMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(p.msgSeq),
		Type: &pb.GameMessage_Join{
			Join: &pb.GameMessage_JoinMsg{
				PlayerType:    pb.PlayerType_HUMAN.Enum(),
				PlayerName:    p.playerInfo.Name,
				GameName:      proto.String(p.announcement.Games[0].GetGameName()),
				RequestedRole: pb.NodeRole_NORMAL.Enum(),
			},
		},
	}
	p.msgSeq++

	data, err := proto.Marshal(joinMsg)
	if err != nil {
		log.Fatalf("Error marshalling joinMessage: %v", err)
		return
	}

	// Отправляем JoinMsg мастеру
	_, err = p.unicastConn.WriteToUDP(data, p.masterAddr)
	if err != nil {
		log.Fatalf("Error sending joinMessage: %v", err)
		return
	}
}
