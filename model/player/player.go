package player

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
		p.handleRoleChangeMessage(msg)
	case *pb.GameMessage_Ping:
		// Отправляем AckMsg в ответ
		p.sendAck(msg)
	default:
		log.Printf("Received unknown message")
	}
}

// Любое сообщение подтверждается отправкой в ответ сообщения AckMsg с таким же msg_seq
func (p *Player) sendAck(msg *pb.GameMessage) {
	ackMsg := &pb.GameMessage{
		MsgSeq:     proto.Int64(msg.GetMsgSeq()),
		SenderId:   proto.Int32(p.playerInfo.GetId()),
		ReceiverId: proto.Int32(msg.GetSenderId()),
		Type: &pb.GameMessage_Ack{
			Ack: &pb.GameMessage_AckMsg{},
		},
	}

	data, err := proto.Marshal(ackMsg)
	if err != nil {
		log.Printf("Error marshalling AckMsg: %v", err)
		return
	}

	_, err = p.unicastConn.WriteToUDP(data, p.masterAddr)
	if err != nil {
		log.Printf("Error sending AckMsg: %v", err)
		return
	}
	log.Printf("Sent AckMsg to %v", p.masterAddr)
}

func (p *Player) handleRoleChangeMessage(msg *pb.GameMessage) {
	roleChangeMsg := msg.GetRoleChange()
	switch {
	case roleChangeMsg.GetReceiverRole() == pb.NodeRole_DEPUTY:
		// DEPUTY
		p.playerInfo.Role = pb.NodeRole_DEPUTY.Enum()
		log.Printf("Assigned as DEPUTY")
	case roleChangeMsg.GetReceiverRole() == pb.NodeRole_MASTER:
		// MASTER
		p.playerInfo.Role = pb.NodeRole_MASTER.Enum()
		log.Printf("Assigned as MASTER")
		// TODO: Implement logic to take over as MASTER
	case roleChangeMsg.GetReceiverRole() == pb.NodeRole_VIEWER:
		// VIEWER
		p.playerInfo.Role = pb.NodeRole_VIEWER.Enum()
		log.Printf("Assigned as VIEWER")
	default:
		log.Printf("Received unknown RoleChangeMsg")
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

func (p *Player) sendSteerMessage() {
	steerMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(p.msgSeq),
		Type: &pb.GameMessage_Steer{
			Steer: &pb.GameMessage_SteerMsg{
				// TODO: поправить направление
				Direction: pb.Direction_UP.Enum(),
			},
		},
	}
	p.msgSeq++

	data, err := proto.Marshal(steerMsg)
	if err != nil {
		log.Fatalf("Error marshalling steerMessage: %v", err)
		return
	}

	_, err = p.unicastConn.WriteToUDP(data, p.masterAddr)
	if err != nil {
		log.Fatalf("Error sending steerMessage: %v", err)
		return
	}
}

func (p *Player) sendPing() {
	pingMsg := &pb.GameMessage{
		MsgSeq:   proto.Int64(p.msgSeq),
		SenderId: proto.Int32(p.playerInfo.GetId()),
		Type: &pb.GameMessage_Ping{
			Ping: &pb.GameMessage_PingMsg{},
		},
	}
	p.msgSeq++

	data, err := proto.Marshal(pingMsg)
	if err != nil {
		log.Printf("Error marshalling PingMsg: %v", err)
		return
	}

	_, err = p.unicastConn.WriteToUDP(data, p.masterAddr)
	if err != nil {
		log.Printf("Error sending PingMsg: %v", err)
		return
	}
}

func (p *Player) sendRoleChangeRequest(newRole pb.NodeRole) {
	roleChangeMsg := &pb.GameMessage{
		MsgSeq:   proto.Int64(p.msgSeq),
		SenderId: proto.Int32(p.playerInfo.GetId()),
		Type: &pb.GameMessage_RoleChange{
			RoleChange: &pb.GameMessage_RoleChangeMsg{
				SenderRole:   p.playerInfo.GetRole().Enum(),
				ReceiverRole: newRole.Enum(),
			},
		},
	}
	p.msgSeq++

	data, err := proto.Marshal(roleChangeMsg)
	if err != nil {
		log.Printf("Error marshalling RoleChangeMsg: %v", err)
		return
	}

	_, err = p.unicastConn.WriteToUDP(data, p.masterAddr)
	if err != nil {
		log.Printf("Error sending RoleChangeMsg: %v", err)
		return
	}
}
