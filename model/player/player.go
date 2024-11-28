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
	lastStateMsg    int64
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
		lastStateMsg:    0,
	}
}

func (p *Player) Start() {
	p.discoverGames()
	go p.receiveMulticastMessages()
	go p.receiveMessages()
	go p.node.ResendUnconfirmedMessages(p.node.Config.GetStateDelayMs())
	go p.node.SendPings(p.node.Config.GetStateDelayMs())
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
	p.node.LastInteraction[msg.GetSenderId()] = time.Now()
	switch t := msg.Type.(type) {
	case *pb.GameMessage_Ack:
		p.node.PlayerInfo.Id = proto.Int32(msg.GetReceiverId())
		p.node.AckChan <- msg.GetMsgSeq()
		log.Printf("Joined game with ID: %d", p.node.PlayerInfo.GetId())
	case *pb.GameMessage_Announcement:
		p.masterAddr = addr
		p.announcementMsg = t.Announcement
		log.Printf("Received AnnouncementMsg from %v via unicast", addr)
		p.sendJoinRequest()
	case *pb.GameMessage_State:
		if msg.GetMsgSeq() <= p.lastStateMsg {
			return
		} else {
			p.lastStateMsg = msg.GetMsgSeq()
		}
		p.node.State = t.State.GetState()
		p.node.SendAck(msg, addr)
		log.Printf("Received StateMsg with state_order: %d", p.node.State.GetStateOrder())
		// Обновить интерфейс пользователя (интегрировать с UI)
	case *pb.GameMessage_Error:
		p.node.SendAck(msg, addr)
		log.Printf("Error from server: %s", t.Error.GetErrorMessage())
	case *pb.GameMessage_RoleChange:
		p.handleRoleChangeMessage(msg)
		p.node.SendAck(msg, addr)
	case *pb.GameMessage_Ping:
		// Отправляем AckMsg в ответ
		p.node.SendAck(msg, addr)
	default:
		log.Printf("Received unknown message")
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

// обработка отвалившихся узлов
//func (p *Player) checkTimeouts() {
//	ticker := time.NewTicker(time.Duration(0.8*float64(p.node.Config.GetStateDelayMs())) * time.Millisecond)
//	defer ticker.Stop()
//
//	for range ticker.C {
//		now := time.Now()
//		p.node.Mu.Lock()
//		for _, lastInteraction := range p.node.LastInteraction {
//			// TODO: добавить проверку на то что мастер отвалился
//			if now.Sub(lastInteraction) > time.Duration(0.8*float64(p.node.Config.GetStateDelayMs()))*time.Millisecond {
//				switch p.node.PlayerInfo.GetRole() {
//				// игрок заметил, что мастер отвалился и переходит к Deputy
//				case pb.NodeRole_NORMAL:
//					deputy := p.getDeputy()
//					if deputy != nil {
//						addrStr := fmt.Sprintf("%s:%d", deputy.GetIpAddress(), deputy.GetPort())
//						addr, err := net.ResolveUDPAddr("udp", addrStr)
//						if err != nil {
//							log.Printf("Error resolving deputy address: %v", err)
//							p.node.Mu.Unlock()
//							continue
//						}
//						p.masterAddr = addr
//						log.Printf("Switched to DEPUTY as new MASTER at %v", p.masterAddr)
//					} else {
//						log.Printf("No DEPUTY available to switch to")
//					}
//
//				// Deputy заметил, что отвалился мастер и заменяет его
//				case pb.NodeRole_DEPUTY:
//					p.becomeMaster()
//				}
//			}
//		}
//		p.node.Mu.Unlock()
//	}
//}
//
//func (p *Player) getDeputy() *pb.GamePlayer {
//	for _, player := range p.node.State.Players.Players {
//		if player.GetRole() == pb.NodeRole_DEPUTY {
//			return player
//		}
//	}
//	return nil
//}
//
//func (p *Player) becomeMaster() {
//	log.Printf("DEPUTY becoming new MASTER")
//	// Обновляем роль игрока
//	p.node.PlayerInfo.Role = pb.NodeRole_MASTER.Enum()
//
//	// Создаем новый мастер
//	masterNode := master.NewDeputyMaster(p.node, p.node.PlayerInfo, p.lastStateMsg)
//	// Запускаем мастер
//	go masterNode.Start()
//	// Останавливаем функции игрока
//	p.stopPlayerFunctions()
//}
