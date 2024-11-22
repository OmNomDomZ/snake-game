package master

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"strconv"
	"time"
)

func (m *Master) handleJoinMessage(joinMsg *pb.GameMessage_JoinMsg, addr *net.UDPAddr) {
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
	m.node.State.Players = m.players

	ackMsg := &pb.GameMessage{
		MsgSeq:     proto.Int64(m.node.MsgSeq),
		SenderId:   proto.Int32(1),
		ReceiverId: proto.Int32(newPlayerID),
		Type: &pb.GameMessage_Ack{
			Ack: &pb.GameMessage_AckMsg{},
		},
	}
	m.node.MsgSeq++

	data, err := proto.Marshal(ackMsg)
	if err != nil {
		log.Printf("Error marshalling AckMsg: %v", err)
		return
	}

	_, err = m.node.UnicastConn.WriteTo(data, addr)
	if err != nil {
		log.Printf("Error sending AckMsg: %v", err)
		return
	}

	m.addSnakeForNewPlayer(newPlayerID)

	m.checkAndAssignDeputy()
}

func (m *Master) checkAndAssignDeputy() {
	if m.hasDeputy() {
		return
	}

	for _, player := range m.players.Players {
		if player.GetRole() == pb.NodeRole_NORMAL {
			m.assignDeputy(player)
			break
		}
	}
}

func (m *Master) hasDeputy() bool {
	for _, player := range m.players.Players {
		if player.GetRole() == pb.NodeRole_DEPUTY {
			return true
		}
	}
	return false
}

func (m *Master) addSnakeForNewPlayer(playerID int32) {
	newSnake := &pb.GameState_Snake{
		PlayerId: proto.Int32(playerID),
		Points: []*pb.GameState_Coord{{X: proto.Int32(m.node.Config.GetWidth() / 2),
			Y: proto.Int32(m.node.Config.GetHeight() / 2)}},
		State:         pb.GameState_Snake_ALIVE.Enum(),
		HeadDirection: pb.Direction_RIGHT.Enum(),
	}

	m.node.State.Snakes = append(m.node.State.Snakes, newSnake)
}

func (m *Master) handleDiscoverMessage(addr *net.UDPAddr) {
	log.Printf("Received DiscoverMsg from %v via unicast", addr)
	announcementMsg := &pb.GameMessage{
		MsgSeq: proto.Int64(m.node.MsgSeq),
		Type: &pb.GameMessage_Announcement{
			Announcement: &pb.GameMessage_AnnouncementMsg{
				Games: []*pb.GameAnnouncement{m.node.Announcement},
			},
		},
	}
	m.node.MsgSeq++

	data, err := proto.Marshal(announcementMsg)
	if err != nil {
		log.Printf("Error marshalling AnnouncementMsg: %v", err)
		return
	}

	_, err = m.node.UnicastConn.WriteToUDP(data, addr)
	if err != nil {
		log.Printf("Error sending AnnouncementMsg: %v", err)
		return
	}
	log.Printf("Sent AnnouncementMsg to %v via unicast", addr)
}

func (m *Master) handleSteerMessage(steerMsg *pb.GameMessage_SteerMsg, playerId int32) {
	var snake *pb.GameState_Snake
	for _, s := range m.node.State.Snakes {
		if s.GetPlayerId() == playerId {
			snake = s
			break
		}
	}

	if snake == nil {
		log.Printf("No snake found for player ID: %d", playerId)
		return
	}

	newDirection := steerMsg.GetDirection()
	currentDirection := snake.GetHeadDirection()

	isOppositeDirection := func(cur, new pb.Direction) bool {
		switch cur {
		case pb.Direction_UP:
			return new == pb.Direction_DOWN
		case pb.Direction_DOWN:
			return new == pb.Direction_UP
		case pb.Direction_LEFT:
			return new == pb.Direction_RIGHT
		case pb.Direction_RIGHT:
			return new == pb.Direction_LEFT
		}
		return false
	}(currentDirection, newDirection)

	if isOppositeDirection {
		log.Printf("Invalid direction change from player ID: %d", playerId)
		return
	}

	snake.HeadDirection = newDirection.Enum()
	log.Printf("Player ID: %d changed direction to: %v", playerId, newDirection)
}

// обработка отвалившихся узлов
func (m *Master) checkTimeouts() {
	ticker := time.NewTicker(time.Duration(0.8*float64(m.node.Config.GetStateDelayMs())) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		for playerId, lastInteraction := range m.lastInteraction {
			if time.Since(lastInteraction) > time.Duration(0.8*float64(m.node.Config.GetStateDelayMs()))*time.Millisecond {
				log.Printf("player ID: %d has timeout", playerId)
				m.removePlayer(playerId)
			}
		}
	}
}

func (m *Master) removePlayer(playerId int32) {
	delete(m.lastInteraction, playerId)

	var removedPlayer *pb.GamePlayer
	for i, player := range m.players.Players {
		if player.GetId() == playerId {
			removedPlayer = player

			if player.GetRole() != pb.NodeRole_VIEWER {
				// Удаляем только если игрок не VIEWER
				m.players.Players = append(m.players.Players[:i], m.players.Players[i+1:]...)
			}
			break
		}
	}

	if removedPlayer == nil {
		log.Printf("Player ID: %d not found for removal", playerId)
		return
	}

	// Если игрок был DEPUTY, назначаем нового
	if removedPlayer.GetRole() == pb.NodeRole_DEPUTY {
		m.findNewDeputy()
	}

	// Если игрок стал VIEWER, переводим его змею в ZOMBIE
	if removedPlayer.GetRole() == pb.NodeRole_VIEWER {
		m.makeSnakeZombie(playerId)
	}
	log.Printf("Player %d processed for removal or role change", playerId)
}

func (m *Master) findNewDeputy() {
	for _, player := range m.players.Players {
		if player.GetRole() == pb.NodeRole_NORMAL {
			m.assignDeputy(player)
			break
		}
	}
}

func (m *Master) assignDeputy(player *pb.GamePlayer) {
	player.Role = pb.NodeRole_DEPUTY.Enum()

	roleChangeMsg := &pb.GameMessage{
		MsgSeq:     proto.Int64(m.node.MsgSeq),
		SenderId:   proto.Int32(m.node.PlayerInfo.GetId()),
		ReceiverId: proto.Int32(player.GetId()),
		Type: &pb.GameMessage_RoleChange{
			RoleChange: &pb.GameMessage_RoleChangeMsg{
				SenderRole:   pb.NodeRole_MASTER.Enum(),
				ReceiverRole: pb.NodeRole_DEPUTY.Enum(),
			},
		},
	}
	m.node.MsgSeq++

	data, err := proto.Marshal(roleChangeMsg)
	if err != nil {
		log.Printf("Error marshalling RoleChangeMsg: %v", err)
		return
	}

	playerAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(player.GetIpAddress(), strconv.Itoa(int(player.GetPort()))))
	if err != nil {
		log.Printf("Error resolving address: %v", err)
		return
	}

	_, err = m.node.UnicastConn.WriteToUDP(data, playerAddr)
	if err != nil {
		log.Printf("Error sending RoleChangeMsgto player ID: %d", player.GetId())
	} else {
		log.Printf("Player ID: %d is new DEPUTY", player.GetId())
	}
}

func (m *Master) makeSnakeZombie(playerId int32) {
	for _, snake := range m.node.State.Snakes {
		if snake.GetPlayerId() == playerId {
			snake.State = pb.GameState_Snake_ZOMBIE.Enum()
			log.Printf("Snake for player ID: %d is now a ZOMBIE", playerId)
			return
		}
	}
	log.Printf("No snake found for player ID: %d to make ZOMBIE", playerId)
}

// обработка roleChangeMsg от Deputy
func (m *Master) handleRoleChangeMessage(msg *pb.GameMessage, addr *net.UDPAddr) {
	roleChangeMsg := msg.GetRoleChange()

	switch {
	case roleChangeMsg.GetSenderRole() == pb.NodeRole_DEPUTY && roleChangeMsg.GetReceiverRole() == pb.NodeRole_MASTER:
		// DEPUTY -> MASTER
		log.Printf("Deputy has taken over as MASTER. Stopping PlayerInfo.")
		m.stopMaster()

	case roleChangeMsg.GetReceiverRole() == pb.NodeRole_VIEWER:
		// Player -> VIEWER
		playerId := msg.GetSenderId()
		log.Printf("Player ID: %d is now a VIEWER. Converting snake to ZOMBIE.", playerId)
		m.makeSnakeZombie(playerId)
	default:
		log.Printf("Received unknown RoleChangeMsg from player ID: %d", msg.GetSenderId())
	}
}

func (m *Master) stopMaster() {
	log.Println("Switching PlayerInfo role to VIEWER...")

	// Меняем роль мастера
	m.node.PlayerInfo.Role = pb.NodeRole_VIEWER.Enum()
	// Делаем змею мастера ZOMBIE
	m.makeSnakeZombie(m.node.PlayerInfo.GetId())

	m.node.Announcement.CanJoin = proto.Bool(false)
	// Останавливаем функции мастера
	log.Println("Master is now a VIEWER. Continuing as an observer.")
}
