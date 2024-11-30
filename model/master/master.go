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
	lastStateMsg int64
}

func NewMaster(multicastConn *net.UDPConn, config *pb.GameConfig) *Master {
	localAddr, err := net.ResolveUDPAddr("udp4", ":0")
	if err != nil {
		log.Fatalf("Error resolving local UDP address: %v", err)
	}
	unicastConn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		log.Fatalf("Error creating unicast socket: %v", err)
	}

	masterIP := unicastConn.LocalAddr().(*net.UDPAddr).IP.String()
	masterPort := unicastConn.LocalAddr().(*net.UDPAddr).Port
	fmt.Printf("Выделенный локальный адрес: %s:%v\n", masterIP, masterPort)

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
		GameName: proto.String("Game"),
	}

	node := common.NewNode(state, config, multicastConn, unicastConn, masterPlayer)

	return &Master{
		Node:         node,
		announcement: announcement,
		players:      players,
		lastStateMsg: 0,
	}
}

func NewDeputyMaster(node *common.Node, newMaster *pb.GamePlayer, lastStateMsg int64) *Master {
	newMaster.Role = pb.NodeRole_MASTER.Enum()

	config := node.Config

	unicastConn := node.UnicastConn

	state := node.State

	announcement := &pb.GameAnnouncement{
		Players:  state.Players,
		Config:   config,
		CanJoin:  proto.Bool(true),
		GameName: proto.String("Game"),
	}

	multicastConn := node.MulticastConn

	newNode := common.NewNode(state, config, multicastConn, unicastConn, newMaster)

	return &Master{
		Node:         newNode,
		announcement: announcement,
		players:      state.Players,
		lastStateMsg: lastStateMsg,
	}
}

func (m *Master) Start() {
	go m.sendAnnouncementMessage()
	go m.receiveMessages()
	go m.receiveMulticastMessages()
	go m.checkTimeouts()
	go m.sendStateMessage()
	go m.Node.ResendUnconfirmedMessages(m.Node.Config.GetStateDelayMs())
	go m.Node.SendPings(m.Node.Config.GetStateDelayMs())
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
	m.Node.LastInteraction[msg.GetSenderId()] = time.Now()
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Join:
		// проверяем есть ли место 5*5 для новой змеи
		if !m.announcement.GetCanJoin() {
			log.Printf("Player can not join")
			log.Printf("Player cannot join: no available space")
			errorMsg := &pb.GameMessage{
				Type: &pb.GameMessage_Error{
					Error: &pb.GameMessage_ErrorMsg{
						ErrorMessage: proto.String("Cannot join: no available space"),
					},
				},
			}
			m.Node.SendMessage(errorMsg, addr)
			m.Node.SendAck(msg, addr)
		} else {
			// обрабатываем joinMsg
			fmt.Printf("Join msg\n")
			joinMsg := t.Join
			m.handleJoinMessage(joinMsg, addr)
		}

		m.Node.SendAck(msg, addr)

	case *pb.GameMessage_Discover:
		m.handleDiscoverMessage(addr)
		fmt.Printf("Discover msg\n")

	case *pb.GameMessage_Steer:
		playerId := msg.GetSenderId()
		fmt.Printf("Steer msg\n")
		if playerId == 0 {
			playerId = m.getPlayerIdByAddress(addr)
		}
		if playerId != 0 {
			m.handleSteerMessage(t.Steer, playerId)
			m.Node.SendAck(msg, addr)
		} else {
			log.Printf("SteerMsg received from unknown address: %v", addr)
		}

	case *pb.GameMessage_RoleChange:
		fmt.Printf("Role change msg\n")
		m.handleRoleChangeMessage(msg, addr)
		m.Node.SendAck(msg, addr)

	case *pb.GameMessage_Ping:
		fmt.Printf("Ping msg\n")
		m.Node.SendAck(msg, addr)

	case *pb.GameMessage_Ack:
		fmt.Printf("Ack msg\n")
		m.Node.AckChan <- msg.GetMsgSeq()

	case *pb.GameMessage_State:
		fmt.Printf("State msg\n")
		if msg.GetMsgSeq() <= m.lastStateMsg {
			return
		} else {
			m.lastStateMsg = msg.GetMsgSeq()
		}
		// рисуем
		m.Node.SendAck(msg, addr)

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
	ticker := time.NewTicker(time.Duration(m.Node.Config.GetStateDelayMs()) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		m.GenerateFood()
		m.UpdateGameState()

		stateMsg := &pb.GameMessage{
			MsgSeq: proto.Int64(m.Node.MsgSeq),
			Type: &pb.GameMessage_State{
				State: &pb.GameMessage_StateMsg{
					State: m.Node.State,
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
		addrStr := fmt.Sprintf("%s:%d", player.GetIpAddress(), player.GetPort())
		fmt.Printf("getAllPlayersUDPAddrs%v\n", addrStr)
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
