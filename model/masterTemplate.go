package model

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
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

	player := &pb.GamePlayer{
		Name:  proto.String("Master"),
		Id:    proto.Int32(1),
		Role:  pb.NodeRole_MASTER.Enum(),
		Type:  pb.PlayerType_HUMAN.Enum(),
		Score: proto.Int32(0),
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

	// создаем сокет для остальных сообщений
	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		log.Fatalf("Error resolving local UDP address: %v", err)
	}
	unicastConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		log.Fatalf("Error creating unicast socket: %v", err)
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
}

func (m *Master) sendAnnouncementMessage() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		msg := &pb.GameMessage{
			MsgSeq: proto.Int64(1),
			Type: &pb.GameMessage_Announcement{
				Announcement: &pb.GameMessage_AnnouncementMsg{
					Games: []*pb.GameAnnouncement{m.announcement},
				},
			},
		}

		data, err := proto.Marshal(msg)
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
		} else {
			log.Printf(msg.String())
		}
	}
}

func (m *Master) receiveMsg() {
	for {
		buf := make([]byte, 4096)
		n, addr, err := m.unicastConn.ReadFromUDP(buf)
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

		m.handleMessage(&msg, addr)
	}
}

func (m *Master) handleMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Discover:
		response := &pb.GameMessage{
			MsgSeq: proto.Int64(m.msgSeq),
			Type: &pb.GameMessage_Announcement{
				Announcement: &pb.GameMessage_AnnouncementMsg{
					Games: []*pb.GameAnnouncement{m.announcement},
				},
			},
		}
		m.msgSeq++

		data, err := proto.Marshal(response)
		if err != nil {
			log.Printf("Error marshalling AnnouncementMsg: %v", err)
			return
		}

		_, err = m.unicastConn.WriteTo(data, addr)
		if err != nil {
			log.Printf("Error sending AnnouncementMsg: %v", err)
			return
		}

	case *pb.GameMessage_Join:
		// проверяем есть ли место 5*5 для новой змеи
		joinMsg := t.Join
		if !m.announcement.GetCanJoin() {
			// отправляем GameMessage_Error
		} else {
			newPlayerID := int32(len(m.players.Players) + 1)
			newPlayer := &pb.GamePlayer{
				Name:  proto.String(joinMsg.GetPlayerName()),
				Id:    proto.Int32(newPlayerID),
				Role:  joinMsg.GetRequestedRole().Enum(),
				Type:  joinMsg.GetPlayerType().Enum(),
				Score: proto.Int32(0),
			}
			m.players.Players = append(m.players.Players, newPlayer)
			m.state.Players = m.players

			response := &pb.GameMessage{
				MsgSeq:     proto.Int64(m.msgSeq),
				SenderId:   proto.Int32(1),
				ReceiverId: proto.Int32(newPlayerID),
				Type: &pb.GameMessage_Ack{
					Ack: &pb.GameMessage_AckMsg{},
				},
			}
			m.msgSeq++

			buf, err := proto.Marshal(response)
			if err != nil {
				log.Printf("Error marshalling AckMsg: %v", err)
				return
			}

			_, err = m.unicastConn.WriteTo(buf, addr)
			if err != nil {
				log.Printf("Error sending AckMsg: %v", err)
				return
			}

			m.addSnakeForNewPlayer(newPlayerID)
		}
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
