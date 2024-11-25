package player

import (
	"SnakeGame/model/common"
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"time"
)

type Player struct {
	node *common.Node

	announcementMsg *pb.GameMessage_AnnouncementMsg
	masterAddr      *net.UDPAddr
	// время последнего сообщения от игрока [playerId]time
	lastInteraction map[int32]time.Time
	// для отслеживания отправок сообщений
	lastSent map[string]time.Time
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

	playerInfo := &pb.GamePlayer{
		Name:  proto.String("Player"),
		Id:    proto.Int32(0),
		Role:  pb.NodeRole_NORMAL.Enum(),
		Type:  pb.PlayerType_HUMAN.Enum(),
		Score: proto.Int32(0),
	}

	node := common.NewNode(nil, nil, multicastConn, unicastConn, playerInfo)

	return &Player{
		node:            node,
		announcementMsg: nil,
		masterAddr:      nil,
		lastInteraction: make(map[int32]time.Time),
		lastSent:        make(map[string]time.Time),
	}
}

func (p *Player) Start() {
	p.discoverGames()
	go p.receiveMulticastMessages()
	go p.receiveMessages()
	go p.node.ResendUnconfirmedMessages(p.node.Config.GetStateDelayMs())
	go p.node.SendPings(p.node.Config.GetStateDelayMs(), p.lastSent)
}

func (p *Player) receiveMulticastMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := p.node.MulticastConn.ReadFromUDP(buf)
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
		p.announcementMsg = t.Announcement
		p.node.Config = t.Announcement.Games[0].GetConfig()
		log.Printf("Received AnnouncementMsg from %v via multicast", addr)
		p.sendJoinRequest()
	default:
		log.Printf("Received unknown multicast message from %v", addr)
	}
}

func (p *Player) receiveMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := p.node.UnicastConn.ReadFromUDP(buf)
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

		log.Printf("Player: Received message: %v from %v", msg, addr)
		p.handleMessage(&msg, addr)
	}
}

func (p *Player) handleMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Ack:
		p.node.PlayerInfo.Id = proto.Int32(msg.GetReceiverId())
		log.Printf("Joined game with ID: %d", p.node.PlayerInfo.GetId())
	case *pb.GameMessage_Announcement:
		p.masterAddr = addr
		p.announcementMsg = t.Announcement
		log.Printf("Received AnnouncementMsg from %v via unicast", addr)
		p.sendJoinRequest()
	case *pb.GameMessage_State:
		p.node.State = t.State.GetState()
		log.Printf("Received StateMsg with state_order: %d", p.node.State.GetStateOrder())
		// Обновить интерфейс пользователя (интегрировать с UI)
	case *pb.GameMessage_Error:
		log.Printf("Error from server: %s", t.Error.GetErrorMessage())
	case *pb.GameMessage_RoleChange:
		p.handleRoleChangeMessage(msg)
	case *pb.GameMessage_Ping:
		// Отправляем AckMsg в ответ
		p.node.SendAck(msg, addr)
	default:
		log.Printf("Received unknown message")
	}
}

func (p *Player) handleRoleChangeMessage(msg *pb.GameMessage) {
	roleChangeMsg := msg.GetRoleChange()
	switch {
	case roleChangeMsg.GetReceiverRole() == pb.NodeRole_DEPUTY:
		// DEPUTY
		p.node.PlayerInfo.Role = pb.NodeRole_DEPUTY.Enum()
		log.Printf("Assigned as DEPUTY")
	case roleChangeMsg.GetReceiverRole() == pb.NodeRole_MASTER:
		// MASTER
		p.node.PlayerInfo.Role = pb.NodeRole_MASTER.Enum()
		log.Printf("Assigned as MASTER")
		// TODO: Implement logic to take over as MASTER
	case roleChangeMsg.GetReceiverRole() == pb.NodeRole_VIEWER:
		// VIEWER
		p.node.PlayerInfo.Role = pb.NodeRole_VIEWER.Enum()
		log.Printf("Assigned as VIEWER")
	default:
		log.Printf("Received unknown RoleChangeMsg")
	}
}

func (p *Player) discoverGames() {
	discoverMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(p.node.MsgSeq),
		Type: &pb.GameMessage_Discover{
			Discover: &pb.GameMessage_DiscoverMsg{},
		},
	}

	multicastAddr, err := net.ResolveUDPAddr("udp", p.node.MulticastAddress)
	if err != nil {
		log.Fatalf("Error resolving multicast address: %v", err)
		return
	}

	p.node.SendMessage(discoverMsg, multicastAddr)
	log.Printf("Player: Sent DiscoverMsg to multicast address %v", multicastAddr)
}

func (p *Player) sendJoinRequest() {
	if p.announcementMsg == nil || len(p.announcementMsg.Games) == 0 {
		log.Printf("Player: No available games to join")
		return
	}

	joinMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(p.node.MsgSeq),
		Type: &pb.GameMessage_Join{
			Join: &pb.GameMessage_JoinMsg{
				PlayerType:    pb.PlayerType_HUMAN.Enum(),
				PlayerName:    p.node.PlayerInfo.Name,
				GameName:      proto.String(p.announcementMsg.Games[0].GetGameName()),
				RequestedRole: pb.NodeRole_NORMAL.Enum(),
			},
		},
	}

	p.node.SendMessage(joinMsg, p.masterAddr)
	log.Printf("Player: Sent JoinMsg to master at %v", p.masterAddr)
}

func (p *Player) sendSteerMessage() {
	steerMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(p.node.MsgSeq),
		Type: &pb.GameMessage_Steer{
			Steer: &pb.GameMessage_SteerMsg{
				// TODO: поправить направление
				Direction: pb.Direction_UP.Enum(),
			},
		},
	}
	p.node.SendMessage(steerMsg, p.masterAddr)
}

//func (p *Player) sendPing() {
//	pingMsg := &pb.GameMessage{
//		MsgSeq:   proto.Int64(p.node.MsgSeq),
//		SenderId: proto.Int32(p.node.PlayerInfo.GetId()),
//		Type: &pb.GameMessage_Ping{
//			Ping: &pb.GameMessage_PingMsg{},
//		},
//	}
//	p.node.MsgSeq++
//
//	data, err := proto.Marshal(pingMsg)
//	if err != nil {
//		log.Printf("Error marshalling PingMsg: %v", err)
//		return
//	}
//
//	_, err = p.node.UnicastConn.WriteToUDP(data, p.masterAddr)
//	if err != nil {
//		log.Printf("Error sending PingMsg: %v", err)
//		return
//	}
//}

func (p *Player) sendRoleChangeRequest(newRole pb.NodeRole) {
	roleChangeMsg := &pb.GameMessage{
		MsgSeq:   proto.Int64(p.node.MsgSeq),
		SenderId: proto.Int32(p.node.PlayerInfo.GetId()),
		Type: &pb.GameMessage_RoleChange{
			RoleChange: &pb.GameMessage_RoleChangeMsg{
				SenderRole:   p.node.PlayerInfo.GetRole().Enum(),
				ReceiverRole: newRole.Enum(),
			},
		},
	}

	p.node.SendMessage(roleChangeMsg, p.masterAddr)
	log.Printf("Player: Sent RoleChangeMsg to %v with new role: %v", p.masterAddr, newRole)
}
