package master

import (
	"SnakeGame/model/common"
	pb "SnakeGame/model/proto"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"time"
)

// TODO: добавить генерацию еды
type Master struct {
	node common.Node

	announcement *pb.GameAnnouncement
	players      *pb.GamePlayers
	// время последнего сообщения от игрока [playerId]time
	lastInteraction map[int32]time.Time
	// для отслеживания отправок сообщений
	lastSent map[string]time.Time
}

func NewMaster(multicastConn *net.UDPConn) *Master {
	config := &pb.GameConfig{
		Width:        proto.Int32(25),
		Height:       proto.Int32(25),
		FoodStatic:   proto.Int32(3),
		StateDelayMs: proto.Int32(200),
	}

	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		log.Fatalf("Error resolving local UDP address: %v", err)
	}
	unicastConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		log.Fatalf("Error creating unicast socket: %v", err)
	}

	masterIP := unicastConn.LocalAddr().(*net.UDPAddr).IP.String()
	masterPort := int32(unicastConn.LocalAddr().(*net.UDPAddr).Port)

	masterPlayer := &pb.GamePlayer{
		Name:      proto.String("Master"),
		Id:        proto.Int32(1),
		Role:      pb.NodeRole_MASTER.Enum(),
		Type:      pb.PlayerType_HUMAN.Enum(),
		Score:     proto.Int32(0),
		IpAddress: proto.String(masterIP),
		Port:      proto.Int32(masterPort),
	}

	players := &pb.GamePlayers{
		Players: []*pb.GamePlayer{masterPlayer},
	}

	state := &pb.GameState{
		StateOrder: proto.Int32(1),
		Snakes:     []*pb.GameState_Snake{},
		Foods:      []*pb.GameState_Coord{},
		Players:    players,
	}

	announcement := &pb.GameAnnouncement{
		Players:  players,
		Config:   config,
		CanJoin:  proto.Bool(true),
		GameName: proto.String("Game"),
	}

	node := common.NewNode(state, config, multicastConn, unicastConn, masterPlayer)

	return &Master{
		node:            node,
		announcement:    announcement,
		players:         players,
		lastInteraction: map[int32]time.Time{},
		lastSent:        make(map[string]time.Time),
	}
}

func (m *Master) Start() {
	go m.sendAnnouncementMessage()
	go m.receiveMessages()
	go m.receiveMulticastMessages()
	go m.checkTimeouts()
	go m.sendStateMessage()
	go m.node.ResendUnconfirmedMessages(m.node.Config.GetStateDelayMs())
	go m.node.SendPings(m.node.Config.GetStateDelayMs(), m.lastSent)
}

// отправка AnnouncementMsg
func (m *Master) sendAnnouncementMessage() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		announcementMsg := &pb.GameMessage{
			MsgSeq: proto.Int64(1),
			Type: &pb.GameMessage_Announcement{
				Announcement: &pb.GameMessage_AnnouncementMsg{
					Games: []*pb.GameAnnouncement{m.announcement},
				},
			},
		}
		multicastAddr, err := net.ResolveUDPAddr("udp", m.node.MulticastAddress)
		if err != nil {
			log.Fatalf("Error resolving multicast address: %v", err)
		}
		m.node.SendMessage(announcementMsg, multicastAddr)
	}
}

// получение мультикаст сообщений
func (m *Master) receiveMulticastMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := m.node.MulticastConn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error receiving multicast message: %v", err)
			continue
		}

		var msg pb.GameMessage
		err = proto.Unmarshal(buf[:n], &msg)
		if err != nil {
			log.Printf("Error unmarshalling multicast message: %v", err)
			continue
		}

		m.handleMulticastMessage(&msg, addr)
	}
}

// обработка мультикаст сообщений
func (m *Master) handleMulticastMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Discover:
		// пришел DiscoverMsg отправляем AnnouncementMsg
		announcementMsg := &pb.GameMessage{
			MsgSeq: proto.Int64(m.node.MsgSeq),
			Type: &pb.GameMessage_Announcement{
				Announcement: &pb.GameMessage_AnnouncementMsg{
					Games: []*pb.GameAnnouncement{m.announcement},
				},
			},
		}
		m.node.SendMessage(announcementMsg, addr)
	default:
		log.Printf("PlayerInfo: Receive unknown multicast message from %v, type %v", addr, t)
	}
}

// получение юникаст сообщений
func (m *Master) receiveMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := m.node.UnicastConn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			continue
		}

		var msg pb.GameMessage
		err = proto.Unmarshal(buf[:n], &msg)
		log.Printf(msg.String())
		if err != nil {
			log.Printf("Error unmarshalling message: %v", err)
			continue
		}

		log.Printf("Master: Received message: %v from %v", msg, addr)
		m.handleMessage(&msg, addr)
	}
}

// обработка юникаст сообщения
func (m *Master) handleMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Join:
		// проверяем есть ли место 5*5 для новой змеи
		if !m.announcement.GetCanJoin() {
			log.Printf("Player can not join")
			// отправляем GameMessage_Error
		} else {
			// обрабатываем joinMsg
			joinMsg := t.Join
			m.handleJoinMessage(joinMsg, addr)
		}

	case *pb.GameMessage_Discover:
		m.handleDiscoverMessage(addr)

	case *pb.GameMessage_Steer:
		playerId := msg.GetSenderId()
		if playerId == 0 {
			playerId = m.getPlayerIdByAddress(addr)
		}
		if playerId != 0 {
			m.handleSteerMessage(t.Steer, playerId)
			m.lastInteraction[playerId] = time.Now()
		} else {
			log.Printf("SteerMsg received from unknown address: %v", addr)
		}

	case *pb.GameMessage_RoleChange:
		m.handleRoleChangeMessage(msg, addr)
		m.lastInteraction[msg.GetSenderId()] = time.Now()

	default:
		log.Printf("Received unknown message type from %v", addr)
	}
}

func (m *Master) getPlayerIdByAddress(addr *net.UDPAddr) int32 {
	for _, player := range m.players.Players {
		if player.GetIpAddress() == addr.IP.String() && int(player.GetPort()) == addr.Port {
			return player.GetId()
		}
	}
	return 0
}

func (m *Master) sendStateMessage() {
	ticker := time.NewTicker(time.Duration(m.node.Config.GetStateDelayMs()) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		m.generateFood()
		m.updateGameState()

		stateMsg := &pb.GameMessage{
			MsgSeq: proto.Int64(m.node.MsgSeq),
			Type: &pb.GameMessage_State{
				State: &pb.GameMessage_StateMsg{
					State: m.node.State,
				},
			},
		}
		m.sendMessageToAllPlayers(stateMsg, m.getAllPlayersUDPAddrs())
	}
}

// получение списка адресов всех игроков (кроме мастера)
func (m *Master) getAllPlayersUDPAddrs() []*net.UDPAddr {
	var addrs []*net.UDPAddr
	for _, player := range m.players.Players {
		if player.GetRole() == pb.NodeRole_MASTER {
			continue
		}
		addrStr := fmt.Sprintf("%s:%d", player.GetIpAddress(), player.GetPort())
		addr, err := net.ResolveUDPAddr("udp", addrStr)
		if err != nil {
			log.Printf("Error resolving UDP address for player ID %d: %v", player.GetId(), err)
			continue
		}
		addrs = append(addrs, addr)
	}
	return addrs
}

// отправка всем игрокам
func (m *Master) sendMessageToAllPlayers(msg *pb.GameMessage, addrs []*net.UDPAddr) {
	for _, addr := range addrs {
		m.node.SendMessage(msg, addr)
	}
}
