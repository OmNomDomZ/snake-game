package model

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"strconv"
	"time"
)

type Master struct {
	state            *pb.GameState
	config           *pb.GameConfig
	players          *pb.GamePlayers
	multicastAddress string
	multicastConn    *net.UDPConn
	unicastConn      *net.UDPConn
	announcement     *pb.GameAnnouncement
	msgSeq           int64
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

	player := &pb.GamePlayer{
		Name:      proto.String("Master"),
		Id:        proto.Int32(1),
		Role:      pb.NodeRole_MASTER.Enum(),
		Type:      pb.PlayerType_HUMAN.Enum(),
		Score:     proto.Int32(0),
		IpAddress: proto.String(masterIP),
		Port:      proto.Int32(masterPort),
	}

	players := &pb.GamePlayers{
		Players: []*pb.GamePlayer{player},
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
		joinMsg := t.Join
		if !m.announcement.GetCanJoin() {
			log.Printf("Player can not join")
			// отправляем GameMessage_Error
		} else {
			newPlayerID := int32(len(m.players.Players) + 1)
			newPlayer := &pb.GamePlayer{
				Name:      proto.String(joinMsg.GetPlayerName()),
				Id:        proto.Int32(newPlayerID),
				IpAddress: proto.String(addr.IP.String()),
				Port:      proto.Int32(int32(addr.Port)),
				Role:      joinMsg.GetRequestedRole().Enum(),
				Type:      joinMsg.GetPlayerType().Enum(),
				Score:     proto.Int32(0),
			}
			m.players.Players = append(m.players.Players, newPlayer)
			m.state.Players = m.players

			ackMsg := &pb.GameMessage{
				MsgSeq:     proto.Int64(m.msgSeq),
				SenderId:   proto.Int32(1),
				ReceiverId: proto.Int32(newPlayerID),
				Type: &pb.GameMessage_Ack{
					Ack: &pb.GameMessage_AckMsg{},
				},
			}
			m.msgSeq++

			data, err := proto.Marshal(ackMsg)
			if err != nil {
				log.Printf("Error marshalling AckMsg: %v", err)
				return
			}

			_, err = m.unicastConn.WriteTo(data, addr)
			if err != nil {
				log.Printf("Error sending AckMsg: %v", err)
				return
			}

			m.addSnakeForNewPlayer(newPlayerID)
		}
	case *pb.GameMessage_Discover:
		log.Printf("Received DiscoverMsg from %v via unicast", addr)
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
			log.Printf("Error marshalling AnnouncementMsg: %v", err)
			return
		}

		_, err = m.unicastConn.WriteToUDP(data, addr)
		if err != nil {
			log.Printf("Error sending AnnouncementMsg: %v", err)
			return
		}
		log.Printf("Sent AnnouncementMsg to %v via unicast", addr)
	default:
		log.Printf("Received unknown message type from %v", addr)
	}
}
func (m *Master) addSnakeForNewPlayer(playerID int32) {
	newSnake := &pb.GameState_Snake{
		PlayerId: proto.Int32(playerID),
		Points: []*pb.GameState_Coord{{X: proto.Int32(m.config.GetWidth() / 2),
			Y: proto.Int32(m.config.GetHeight() / 2)}},
		State:         pb.GameState_Snake_ALIVE.Enum(),
		HeadDirection: pb.Direction_RIGHT.Enum(),
	}

	m.state.Snakes = append(m.state.Snakes, newSnake)
}

func (m *Master) sendStateMessage() {
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
