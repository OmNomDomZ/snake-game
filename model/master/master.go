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

type Master struct {
	Node *common.Node

	announcement *pb.GameAnnouncement
	players      *pb.GamePlayers
	lastStateMsg int32
}

// NewMaster создает нового мастера
func NewMaster(multicastConn *net.UDPConn, config *pb.GameConfig) *Master {
	localAddr, err := net.ResolveUDPAddr("udp4", ":0")
	if err != nil {
		log.Fatalf("Error resolving local UDP address: %v", err)
	}
	unicastConn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		log.Fatalf("Error creating unicast socket: %v", err)
	}

	masterIP, err := getLocalIP()
	if err != nil {
		log.Fatalf("Error getting local IP: %v", err)
	}
	masterPort := unicastConn.LocalAddr().(*net.UDPAddr).Port
	log.Printf("Выделенный локальный адрес: %s:%v\n", masterIP, masterPort)

	masterPlayer := &pb.GamePlayer{
		Name:      proto.String("Master"),
		Id:        proto.Int32(1),
		Role:      pb.NodeRole_MASTER.Enum(),
		Type:      pb.PlayerType_HUMAN.Enum(),
		Score:     proto.Int32(0),
		IpAddress: proto.String(masterIP),
		Port:      proto.Int32(int32(masterPort)),
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

	masterSnake := &pb.GameState_Snake{
		PlayerId: proto.Int32(masterPlayer.GetId()),
		Points: []*pb.GameState_Coord{
			{
				X: proto.Int32(config.GetWidth() / 2),
				Y: proto.Int32(config.GetHeight() / 2),
			},
		},
		State:         pb.GameState_Snake_ALIVE.Enum(),
		HeadDirection: pb.Direction_RIGHT.Enum(),
	}

	state.Snakes = append(state.Snakes, masterSnake)

	announcement := &pb.GameAnnouncement{
		Players:  players,
		Config:   config,
		CanJoin:  proto.Bool(true),
		GameName: proto.String("Game1"),
	}

	node := common.NewNode(state, config, multicastConn, unicastConn, masterPlayer)

	return &Master{
		Node:         node,
		announcement: announcement,
		players:      players,
		lastStateMsg: 0,
	}
}

// для получения реального ip
func getLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("error getting network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// IPv4
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("no connected network interface found")
}

//func NewDeputyMaster(node *common.Node, newMaster *pb.GamePlayer, lastStateMsg int64) *Master {
//	newMaster.Role = pb.NodeRole_MASTER.Enum()
//
//	config := node.Config
//
//	unicastConn := node.UnicastConn
//
//	state := node.State
//
//	announcement := &pb.GameAnnouncement{
//		Players:  state.Players,
//		Config:   config,
//		CanJoin:  proto.Bool(true),
//		GameName: proto.String("Game"),
//	}
//
//	multicastConn := node.MulticastConn
//
//	newNode := common.NewNode(state, config, multicastConn, unicastConn, newMaster)
//
//	return &Master{
//		Node:         newNode,
//		announcement: announcement,
//		players:      state.Players,
//		lastStateMsg: lastStateMsg,
//	}
//}

// Start запуск мастера
func (m *Master) Start() {
	go m.sendAnnouncementMessage()
	go m.receiveMessages()
	go m.receiveMulticastMessages()
	//go m.checkTimeouts()
	go m.sendStateMessage()
	//go m.Node.ResendUnconfirmedMessages(m.Node.Config.GetStateDelayMs())
	//go m.Node.SendPings(m.Node.Config.GetStateDelayMs())
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
		multicastAddr, err := net.ResolveUDPAddr("udp", m.Node.MulticastAddress)
		if err != nil {
			log.Fatalf("Error resolving multicast address: %v", err)
		}
		m.Node.SendMessage(announcementMsg, multicastAddr)
	}
}

// получение мультикаст сообщений
func (m *Master) receiveMulticastMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := m.Node.MulticastConn.ReadFromUDP(buf)
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
			MsgSeq: proto.Int64(m.Node.MsgSeq),
			Type: &pb.GameMessage_Announcement{
				Announcement: &pb.GameMessage_AnnouncementMsg{
					Games: []*pb.GameAnnouncement{m.announcement},
				},
			},
		}
		m.Node.SendMessage(announcementMsg, addr)
	default:
		log.Printf("PlayerInfo: Receive unknown multicast message from %v, type %v", addr, t)
	}
}

// получение юникаст сообщений
func (m *Master) receiveMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := m.Node.UnicastConn.ReadFromUDP(buf)
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

		if m.Node.PlayerInfo.GetIpAddress() == addr.IP.String() && m.Node.PlayerInfo.GetPort() == int32(addr.Port) {
			log.Printf("Get msg from itself")
			continue
		}
		log.Printf("Master: Received message: %v from %v", msg.String(), addr)
		m.handleMessage(&msg, addr)
	}
}

// обработка юникаст сообщения
func (m *Master) handleMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	m.Node.LastInteraction[msg.GetSenderId()] = time.Now()
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Join:
		// проверяем есть ли место 5*5 для новой змеи
		hasSquare, coord := m.hasFreeSquare(m.Node.State, m.Node.Config, 5)

		if !hasSquare {
			m.announcement.CanJoin = proto.Bool(false)
			m.handleErrorMsg(addr)
			log.Printf("Player cannot join: no available space")
			//m.Node.SendAck(msg, addr)
		} else {
			// обрабатываем joinMsg
			log.Printf("Join msg")
			joinMsg := t.Join
			m.handleJoinMessage(joinMsg, addr, coord)
		}

	case *pb.GameMessage_Discover:
		m.handleDiscoverMessage(addr)
		log.Printf("Discover msg")

	case *pb.GameMessage_Steer:
		playerId := msg.GetSenderId()
		log.Printf("Steer msg")
		if playerId == 0 {
			playerId = m.getPlayerIdByAddress(addr)
		}
		if playerId != 0 {
			m.handleSteerMessage(t.Steer, playerId)
			//m.Node.SendAck(msg, addr)
		} else {
			log.Printf("SteerMsg received from unknown address: %v", addr)
		}

	case *pb.GameMessage_RoleChange:
		log.Printf("Role change msg")
		m.handleRoleChangeMessage(msg, addr)
		//m.Node.SendAck(msg, addr)

	case *pb.GameMessage_Ping:
		log.Printf("Ping msg")
		//m.Node.SendAck(msg, addr)

	case *pb.GameMessage_Ack:
		log.Printf("Ack msg")
		//m.Node.AckChan <- msg.GetMsgSeq()

	case *pb.GameMessage_State:
		log.Printf("State msg")
		if t.State.GetState().GetStateOrder() <= m.lastStateMsg {
			return
		} else {
			m.lastStateMsg = t.State.GetState().GetStateOrder()
		}
		//m.Node.SendAck(msg, addr)

	default:
		log.Printf("Received unknown message type from %v", addr)
	}
}

// id игрока по адресу
func (m *Master) getPlayerIdByAddress(addr *net.UDPAddr) int32 {
	for _, player := range m.players.Players {
		if player.GetIpAddress() == addr.IP.String() && int(player.GetPort()) == addr.Port {
			return player.GetId()
		}
	}
	return 0
}

// рассылаем всем игрокам состояние игры
func (m *Master) sendStateMessage() {
	ticker := time.NewTicker(time.Duration(m.Node.Config.GetStateDelayMs()) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		m.GenerateFood()
		m.UpdateGameState()

		newStateOrder := m.Node.State.GetStateOrder() + 1
		m.Node.State.StateOrder = proto.Int32(newStateOrder)

		stateMsg := &pb.GameMessage{
			MsgSeq: proto.Int64(m.Node.MsgSeq),
			Type: &pb.GameMessage_State{
				State: &pb.GameMessage_StateMsg{
					State: &pb.GameState{
						StateOrder: proto.Int32(newStateOrder),
						Snakes:     m.Node.State.GetSnakes(),
						Foods:      m.Node.State.GetFoods(),
						Players:    m.Node.State.GetPlayers(),
					},
				},
			},
		}
		allAddrs := m.getAllPlayersUDPAddrs()
		m.sendMessageToAllPlayers(stateMsg, allAddrs)
	}
}

// получение списка адресов всех игроков (кроме мастера)
func (m *Master) getAllPlayersUDPAddrs() []*net.UDPAddr {
	var addrs []*net.UDPAddr
	for _, player := range m.players.Players {
		// Исключаем самого мастера из списка получателей
		if player.GetId() == m.Node.PlayerInfo.GetId() {
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
		m.Node.SendMessage(msg, addr)
	}
}
