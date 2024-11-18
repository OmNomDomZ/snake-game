package model

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"time"
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
	go p.receiveMessages()
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
		// Обработка AckMsg
		log.Printf("Received AckMsg from %v", addr)
	case *pb.GameMessage_State:
		// Обработка StateMsg
		p.state = t.State.GetState()
		log.Printf("Received StateMsg with state_order: %d", p.state.GetStateOrder())
	case *pb.GameMessage_Error:
		// Обработка ErrorMsg
		log.Printf("Error from server: %s", t.Error.GetErrorMessage())
	case *pb.GameMessage_RoleChange:
		// Обработка RoleChangeMsg
		log.Printf("Received RoleChangeMsg")
	default:
		log.Printf("Received unknown message type from %v", addr)
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

	buf := make([]byte, 4096)
	p.unicastConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, addr, err := p.unicastConn.ReadFromUDP(buf)
	if err != nil {
		log.Printf("Error receiving AnnouncmentMsg message: %v", err)
		return
	}

	var msg pb.GameMessage
	err = proto.Unmarshal(buf[:n], &msg)
	if err != nil {
		log.Printf("Error unmarshalling AnnouncementMsg message: %v", err)
		return
	}

	if _, ok := msg.Type.(*pb.GameMessage_Announcement); ok {
		p.masterAddr = addr
		log.Printf("Discovered game at %v", addr)
		p.sendJoinRequest()
	}
}

func (p *Player) sendJoinRequest() {
	joinMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(p.msgSeq),
		Type: &pb.GameMessage_Join{
			Join: &pb.GameMessage_JoinMsg{
				PlayerType:    pb.PlayerType_HUMAN.Enum(),
				PlayerName:    p.playerInfo.Name,
				GameName:      proto.String("Game"),
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

	_, err = p.unicastConn.WriteToUDP(data, p.masterAddr)
	if err != nil {
		log.Fatalf("Error sending joinMessage: %v", err)
		return
	}

	// ждем AckMsg в ответ
	buf := make([]byte, 4096)
	p.unicastConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := p.unicastConn.ReadFromUDP(buf)
	if err != nil {
		log.Printf("Error receiving AckMsg: %v", err)
		return
	}

	var msg pb.GameMessage
	err = proto.Unmarshal(buf[:n], &msg)
	if err != nil {
		log.Printf("Error unmarshalling AckMsg: %v", err)
		return
	}

	if _, ok := msg.Type.(*pb.GameMessage_Join); ok {
		p.playerInfo.Id = proto.Int32(msg.GetReceiverId())
		log.Printf("Joined game with ID: %d", p.playerInfo.GetId())
	} else {
		log.Printf("Unexpected message type after JoinMsg")
	}
}
