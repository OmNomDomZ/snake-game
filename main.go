package main

import (
	"SnakeGame/connection"
	"SnakeGame/model/master"
	"SnakeGame/model/player"
	"time"
)

func main() {
	// TODO: добавить mapper для proto messages
	multicastConn := connection.Connection()
	defer multicastConn.Close()

	// Запускаем мастера
	master := master.NewMaster(multicastConn)
	master.Start()

	// Задержка, чтобы мастер успел запуститься
	time.Sleep(1 * time.Second)

	// Запускаем игрока
	player := player.NewPlayer(multicastConn)
	player.Start()

	// Даем им пообщаться
	time.Sleep(30 * time.Second)
}
