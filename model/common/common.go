package common

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"sync"
	"time"
)

const MulticastAddr = "239.192.0.4:9192"

// MessageEntry структура для отслеживания неподтвержденных сообщений
type MessageEntry struct {
	msg       *pb.GameMessage
	addr      *net.UDPAddr
	timestamp time.Time
}

// Node общая структура для хранения информации об игроке или мастере
type Node struct {
	State            *pb.GameState
	Config           *pb.GameConfig
	MulticastAddress string
	MulticastConn    *net.UDPConn
	UnicastConn      *net.UDPConn
	PlayerInfo       *pb.GamePlayer
	MsgSeq           int64
	Role             pb.NodeRole

	// время последнего сообщения от игрока [playerId]time
	LastInteraction map[int32]time.Time
	// время отправки последнего сообщения игроку отправок сообщений
	LastSent map[string]time.Time

	unconfirmedMessages map[int64]*MessageEntry
	Mu                  sync.Mutex
	Cond                *sync.Cond
	AckChan             chan int64
}

func NewNode(state *pb.GameState, config *pb.GameConfig, multicastConn *net.UDPConn,
	unicastConn *net.UDPConn, playerInfo *pb.GamePlayer) *Node {
	node := &Node{
		State:            state,
		Config:           config,
		MulticastAddress: MulticastAddr,
		MulticastConn:    multicastConn,
		UnicastConn:      unicastConn,
		PlayerInfo:       playerInfo,
		MsgSeq:           1,

		LastInteraction:     make(map[int32]time.Time),
		LastSent:            make(map[string]time.Time),
		unconfirmedMessages: make(map[int64]*MessageEntry),
		AckChan:             make(chan int64),
	}

	node.Cond = sync.NewCond(&node.Mu)

	return node
}

// SendAck любое сообщение подтверждается отправкой в ответ сообщения AckMsg с таким же msg_seq
func (n *Node) SendAck(msg *pb.GameMessage, addr *net.UDPAddr) {
	switch msg.Type.(type) {
	case *pb.GameMessage_Announcement, *pb.GameMessage_Discover, *pb.GameMessage_Ack:
		return
	}

	id := n.GetPlayerIdByAddress(addr)

	ackMsg := &pb.GameMessage{
		MsgSeq:     proto.Int64(msg.GetMsgSeq()),
		SenderId:   proto.Int32(n.PlayerInfo.GetId()),
		ReceiverId: proto.Int32(id),
		Type: &pb.GameMessage_Ack{
			Ack: &pb.GameMessage_AckMsg{},
		},
	}

	n.SendMessage(ackMsg, addr)

	switch msg.Type.(type) {
	case *pb.GameMessage_Ping:
		log.Printf("Sent AckMsg to %v in response to Ping", id)
	case *pb.GameMessage_Steer:
		log.Printf("Sent AckMsg to %v in response to Steer", id)
	case *pb.GameMessage_State:
		log.Printf("Sent AckMsg to %v in response to State", id)
	case *pb.GameMessage_Join:
		log.Printf("Sent AckMsg to %v in response to Join", id)
	case *pb.GameMessage_Error:
		log.Printf("Sent AckMsg to %v in response to Error", id)
	case *pb.GameMessage_RoleChange:
		log.Printf("Sent AckMsg to %v in response to RoleChange", id)
	}
}

// GetPlayerIdByAddress id игрока по адресу
func (n *Node) GetPlayerIdByAddress(addr *net.UDPAddr) int32 {
	if n.State == nil {
		return 0
	}
	for _, player := range n.State.Players.Players {
		if player.GetIpAddress() == addr.IP.String() && int(player.GetPort()) == addr.Port {
			return player.GetId()
		}
	}
	return -1
}

// SendPing отправка
//func (n *Node) SendPing(addr *net.UDPAddr) {
//	pingMsg := &pb.GameMessage{
//		MsgSeq:   proto.Int64(n.MsgSeq),
//		SenderId: proto.Int32(n.PlayerInfo.GetId()),
//		Type: &pb.GameMessage_Ping{
//			Ping: &pb.GameMessage_PingMsg{},
//		},
//	}
//
//	n.SendMessage(pingMsg, addr)
//}

// SendMessage отправка сообщения и добавление его в неподтверждённые
func (n *Node) SendMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	// увеличиваем порядковый номер сообщения
	//n.Mu.Lock()
	switch msg.Type.(type) {
	case *pb.GameMessage_Ack:

	default:
		msg.MsgSeq = proto.Int64(n.MsgSeq)
		n.MsgSeq++
	}
	//n.Mu.Unlock()

	// отправляем
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error marshalling Message: %v", err)
		return
	}

	_, err = n.UnicastConn.WriteToUDP(data, addr)
	if err != nil {
		log.Printf("Error sending Message: %v", err)
		return
	}

	// добавляем сообщение в неподтверждённые
	//n.Mu.Lock()
	//defer n.Mu.Unlock()
	//if !(n.PlayerInfo.GetIpAddress() == addr.IP.String() && n.PlayerInfo.GetPort() == int32(addr.Port)) {
	//switch msg.Type.(type) {
	//case *pb.GameMessage_Announcement, *pb.GameMessage_Discover, *pb.GameMessage_Ack:
	//
	//default:
	//	n.Mu.Lock()
	//	n.unconfirmedMessages[msg.GetMsgSeq()] = &MessageEntry{
	//		msg:       msg,
	//		addr:      addr,
	//		timestamp: time.Now(),
	//	}
	//
	//	//log.Printf("Add msg to unconfirmed messages %v", msg.GetMsgSeq())
	//	n.Mu.Unlock()
	//}
	//}

	//ip := addr.IP
	//port := addr.Port
	//address := fmt.Sprintf("%s:%d", ip, port)
	//n.LastSent[address] = time.Now()
	//log.Printf("Sent message %v with Seq: %d to %v from %v", msg.Type, msg.GetMsgSeq(), addr, n.PlayerInfo.GetIpAddress()+":"+strconv.Itoa(int(n.PlayerInfo.GetPort())))

	//if n.PlayerInfo.GetIpAddress() == addr.IP.String() &&
	//	n.PlayerInfo.GetPort() == int32(addr.Port) {
	//	n.LastSent[address] = time.Now()
	//}
}

// HandleAck обработка полученных AckMsg
func (n *Node) HandleAck(seq int64) {
	n.Mu.Lock()
	defer n.Mu.Unlock()
	if _, exists := n.unconfirmedMessages[seq]; exists {
		delete(n.unconfirmedMessages, seq)
		log.Printf("Received Ack for Seq: %d", seq)
	}
}

// ResendUnconfirmedMessages проверка и переотправка неподтвержденных сообщений
//func (n *Node) ResendUnconfirmedMessages(stateDelayMs int32) {
//	ticker := time.NewTicker(time.Duration(stateDelayMs/10) * time.Millisecond)
//	defer ticker.Stop()
//
//	for {
//		select {
//		// ответ не пришел, заново отправляем сообщение
//		case <-ticker.C:
//			now := time.Now()
//			n.Mu.Lock()
//			for seq, entry := range n.unconfirmedMessages {
//				if now.Sub(entry.timestamp) > time.Duration(n.Config.GetStateDelayMs()/10)*time.Millisecond {
//					// переотправка сообщения
//					data, err := proto.Marshal(entry.msg)
//					if err != nil {
//						log.Printf("Error marshalling Message: %v", err)
//						continue
//					}
//					_, err = n.UnicastConn.WriteToUDP(data, entry.addr)
//					if err != nil {
//						fmt.Printf("Error sending Message: %v", err)
//						continue
//					}
//
//					entry.timestamp = time.Now()
//					log.Printf("Resent message with Seq: %d to %v from %v", seq, entry.addr, n.PlayerInfo.GetIpAddress()+":"+strconv.Itoa(int(n.PlayerInfo.GetPort())))
//					//log.Printf(entry.msg.String())
//				}
//			}
//			n.Mu.Unlock()
//		// ответ пришел, удаляем из мапы
//		case seq := <-n.AckChan:
//			log.Printf("Get seq %v", seq)
//			n.Mu.Lock()
//			defer n.Mu.Unlock()
//			n.HandleAck(seq)
//		}
//	}
//}

// SendPings отправка PingMsg, если не было отправлено сообщений в течение stateDelayMs/10
//func (n *Node) SendPings(stateDelayMs int32) {
//	ticker := time.NewTicker(time.Duration(stateDelayMs/10) * time.Millisecond)
//	defer ticker.Stop()
//
//	for range ticker.C {
//		now := time.Now()
//		n.Mu.Lock()
//		if n.State == nil {
//			n.Mu.Unlock()
//			return
//		}
//		for _, player := range n.State.Players.Players {
//			if player.GetId() == n.PlayerInfo.GetId() {
//				continue
//			}
//			addrKey := fmt.Sprintf("%s:%d", player.GetIpAddress(), player.GetPort())
//			last, exists := n.LastSent[addrKey]
//			if !exists || now.Sub(last) > time.Duration(n.Config.GetStateDelayMs()/10)*time.Millisecond {
//				playerAddr, err := net.ResolveUDPAddr("udp", addrKey)
//				if err != nil {
//					log.Printf("Error resolving address for Ping: %v", err)
//					continue
//				}
//				n.SendPing(playerAddr)
//				n.LastSent[addrKey] = now
//			}
//		}
//		n.Mu.Unlock()
//	}
//}
