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

// структура для отслеживания неподтвержденных сообщений
type MessageEntry struct {
	msg       *pb.GameMessage
	addr      *net.UDPAddr
	timestamp time.Time
}

// общая структура для хранения информации об игроке или мастере
type Node struct {
	State            *pb.GameState
	Config           *pb.GameConfig
	MulticastAddress string
	MulticastConn    *net.UDPConn
	UnicastConn      *net.UDPConn
	PlayerInfo       *pb.GamePlayer
	MsgSeq           int64

	unconfirmedMessages map[int64]*MessageEntry
	mu                  sync.Mutex
	ackChan             chan int64
}

func NewNode(state *pb.GameState, config *pb.GameConfig, multicastConn *net.UDPConn,
	unicastConn *net.UDPConn, playerInfo *pb.GamePlayer) Node {
	return Node{
		State:               state,
		Config:              config,
		MulticastAddress:    MulticastAddr,
		MulticastConn:       multicastConn,
		UnicastConn:         unicastConn,
		PlayerInfo:          playerInfo,
		MsgSeq:              1,
		unconfirmedMessages: make(map[int64]*MessageEntry),
		ackChan:             make(chan int64),
	}
}

// Любое сообщение подтверждается отправкой в ответ сообщения AckMsg с таким же msg_seq
func (n *Node) SendAck(msg *pb.GameMessage, addr *net.UDPAddr) {
	ackMsg := &pb.GameMessage{
		MsgSeq:     proto.Int64(msg.GetMsgSeq()),
		SenderId:   proto.Int32(n.PlayerInfo.GetId()),
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

	_, err = n.UnicastConn.WriteToUDP(data, addr)
	if err != nil {
		log.Printf("Error sending AckMsg: %v", err)
		return
	}
	log.Printf("Sent AckMsg to %v", addr)
}

// SendPing отправка
func (n *Node) SendPing(addr *net.UDPAddr) {
	pingMsg := &pb.GameMessage{
		MsgSeq:   proto.Int64(n.MsgSeq),
		SenderId: proto.Int32(n.PlayerInfo.GetId()),
		Type: &pb.GameMessage_Ping{
			Ping: &pb.GameMessage_PingMsg{},
		},
	}
	n.MsgSeq++

	data, err := proto.Marshal(pingMsg)
	if err != nil {
		log.Printf("Error marshalling PingMsg: %v", err)
		return
	}

	_, err = n.UnicastConn.WriteToUDP(data, addr)
	if err != nil {
		log.Printf("Error sending PingMsg: %v", err)
		return
	}
}

// SendMessage отправка сообщения и добавление его в неподтверждённые
func (n *Node) SendMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	// увеличиваем порядковый номер сообщения
	n.mu.Lock()
	msg.MsgSeq = proto.Int64(msg.GetMsgSeq() + 1)
	n.mu.Unlock()

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
	n.mu.Lock()
	n.unconfirmedMessages[msg.GetMsgSeq()] = &MessageEntry{
		msg:       msg,
		addr:      addr,
		timestamp: time.Now(),
	}
	n.mu.Unlock()

	log.Printf("Sent message with Seq: %d to %v", msg.GetMsgSeq(), addr)
}

// HandleAck обработка полученных AckMsg
func (n *Node) HandleAck(seq int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.unconfirmedMessages[seq]; exists {
		delete(n.unconfirmedMessages, seq)
		log.Printf("Received Ack for Seq: %d", seq)
	}
}

// ResendUnconfirmedMessages проверка и переотправка неподтвержденных сообщений
func (n *Node) ResendUnconfirmedMessages(stateDelayMs int32) {
	ticker := time.NewTicker(time.Duration(stateDelayMs/10) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			//now := time.Now()
			n.mu.Lock()
		case seq := <-n.ackChan:
			n.HandleAck(seq)
		}
	}
}

//// обработка SteerMsg
//func (n *Node) HandleSteerMessage(steerMsg *pb.GameMessage_SteerMsg, playerId int32) {
//	var snake *pb.GameState_Snake
//	for _, s := range n.State.Snakes {
//		if s.GetPlayerId() == playerId {
//			snake = s
//			break
//		}
//	}
//
//	if snake == nil {
//		log.Printf("No snake found for player ID: %d", playerId)
//		return
//	}
//
//	newDirection := steerMsg.GetDirection()
//	currentDirection := snake.GetHeadDirection()
//
//	isOppositeDirection := func(cur, new pb.Direction) bool {
//		switch cur {
//		case pb.Direction_UP:
//			return new == pb.Direction_DOWN
//		case pb.Direction_DOWN:
//			return new == pb.Direction_UP
//		case pb.Direction_LEFT:
//			return new == pb.Direction_RIGHT
//		case pb.Direction_RIGHT:
//			return new == pb.Direction_LEFT
//		}
//		return false
//	}(currentDirection, newDirection)
//
//	if isOppositeDirection {
//		log.Printf("Invalid direction change from player ID: %d", playerId)
//		return
//	}
//
//	snake.HeadDirection = newDirection.Enum()
//	log.Printf("Player ID: %d changed direction to: %v", playerId, newDirection)
//}
