package master

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"strconv"
	"time"
)

// TODO: добавить генерацию еды
type Master struct {
	state            *pb.GameState
	config           *pb.GameConfig
	master           *pb.GamePlayer
	players          *pb.GamePlayers
	multicastAddress string
	multicastConn    *net.UDPConn
	unicastConn      *net.UDPConn
	announcement     *pb.GameAnnouncement
	msgSeq           int64
	// время последнего сообщения от игрока [playerId]time
	lastInteraction map[int32]time.Time
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

	master := &pb.GamePlayer{
		Name:      proto.String("Master"),
		Id:        proto.Int32(1),
		Role:      pb.NodeRole_MASTER.Enum(),
		Type:      pb.PlayerType_HUMAN.Enum(),
		Score:     proto.Int32(0),
		IpAddress: proto.String(masterIP),
		Port:      proto.Int32(masterPort),
	}

	players := &pb.GamePlayers{
		Players: []*pb.GamePlayer{master},
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

	return &Master{
		state:            state,
		config:           config,
		master:           master,
		players:          players,
		multicastAddress: "239.192.0.4:9192",
		multicastConn:    multicastConn,
		unicastConn:      unicastConn,
		announcement:     announcement,
		msgSeq:           1,
	}
}

func (m *Master) Start() {
	go m.sendAnnouncementMessage()
	go m.receiveMessages()
	go m.receiveMulticastMessages()
	go m.checkTimeouts()
	go m.sendStateMessage()
	go m.sendPingMessage()
}

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
		m.msgSeq++

		data, err := proto.Marshal(announcementMsg)
		if err != nil {
			log.Printf("Error marshalling AnnouncementMsg: %v", err)
			continue
		}
		multicastUDPAddr, err := net.ResolveUDPAddr("udp4", m.multicastAddress)
		if err != nil {
			log.Printf("Error resolving multicast address: %v", err)
		}

		_, err = m.unicastConn.WriteTo(data, multicastUDPAddr)
		if err != nil {
			log.Printf("Error sending AnnouncementMsg: %v", err)
		}
	}
}

func (m *Master) receiveMulticastMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := m.multicastConn.ReadFromUDP(buf)
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

func (m *Master) handleMulticastMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {

	case *pb.GameMessage_Discover:
		// пришел DiscoverMsg отправляем AnnouncementMsg
		announcementMsg := &pb.GameMessage{
			MsgSeq: proto.Int64(m.msgSeq),
			Type: &pb.GameMessage_Announcement{
				Announcement: &pb.GameMessage_AnnouncementMsg{
					Games: []*pb.GameAnnouncement{m.announcement},
				},
			},
		}
		m.msgSeq++

		data, err := proto.Marshal(announcementMsg)
		if err != nil {
			log.Fatalf("Error marshaling response: %v", err)
			return
		}

		_, err = m.unicastConn.WriteToUDP(data, addr)
		if err != nil {
			log.Fatalf("Error sending response: %v", err)
			return
		}
	default:
		log.Printf("master: Receive unknown multicast message from %v, type %v", addr, t)
	}
}

func (m *Master) receiveMessages() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := m.unicastConn.ReadFromUDP(buf)
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

		m.handleMessage(&msg, addr)
	}
}

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
		} else {
			log.Printf("SteerMsg received from unknown address: %v", addr)
		}

	case *pb.GameMessage_RoleChange:
		roleChangeMsg := t.RoleChange
		m.handleRoleChangeMessage(roleChangeMsg)

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
	ticker := time.NewTicker(time.Duration(m.config.GetStateDelayMs()) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		stateMsg := &pb.GameMessage{
			MsgSeq: proto.Int64(m.msgSeq),
			Type: &pb.GameMessage_State{
				State: &pb.GameMessage_StateMsg{
					State: m.state,
				},
			},
		}
		m.msgSeq++

		data, err := proto.Marshal(stateMsg)
		if err != nil {
			log.Printf("Error marshalling StateMsg: %v", err)
			return
		}

		for _, player := range m.players.Players {
			if player.GetRole() == pb.NodeRole_MASTER {
				continue
			}

			playerIp := player.GetIpAddress()
			playerPort := player.GetPort()
			playerAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(playerIp, strconv.Itoa(int(playerPort))))
			if err != nil {
				log.Printf("Error resolving address: %v", err)
				continue
			}

			_, err = m.unicastConn.WriteToUDP(data, playerAddr)
			if err != nil {
				log.Printf("Error sending StateMsg: %v to player (ID: %d)", err, player.GetId())
			} else {
				log.Printf("Sent StateMsg to player (ID: %d)", player.GetId())
			}
		}
	}
}

func (m *Master) sendPingMessage() {
	ticker := time.NewTicker(time.Duration(m.config.GetStateDelayMs()/10) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		for _, player := range m.players.Players {
			if player.GetRole() == pb.NodeRole_MASTER {
				continue
			}

			playerId := player.GetId()
			playerIp := player.GetIpAddress()
			playerPort := player.GetPort()

			playerAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(playerIp, strconv.Itoa(int(playerPort))))
			if err != nil {
				log.Printf("Error resolving address: %v", err)
				continue
			}

			lastInteraction, ok := m.lastInteraction[playerId]
			if !ok {
				log.Printf("player ID: %d is not in lastInteraction", playerId)
				continue
			}

			if time.Since(lastInteraction) > time.Duration(m.config.GetStateDelayMs()/10)*time.Millisecond {
				pingMsg := &pb.GameMessage{
					MsgSeq: proto.Int64(m.msgSeq),
					Type: &pb.GameMessage_Ping{
						Ping: &pb.GameMessage_PingMsg{},
					},
				}
				m.msgSeq++

				data, err := proto.Marshal(pingMsg)
				if err != nil {
					log.Printf("Error marshalling PingMsg: %v", err)
					continue
				}

				_, err = m.unicastConn.WriteToUDP(data, playerAddr)
				if err != nil {
					log.Printf("Error sending PingMsg: %v", err)
				} else {
					log.Printf("Sent Ping Msg to player (ID: %d)", playerId)
				}
			}
		}
	}
}
