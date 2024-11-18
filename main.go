package main

import (
	"SnakeGame/connection"
	"SnakeGame/model"
	"time"
)

func main() {
	multicastConn := connection.Connection()
	defer multicastConn.Close()

	// Запускаем мастера
	master := model.NewMaster(multicastConn)
	master.Start()

	// Задержка, чтобы мастер успел запуститься
	time.Sleep(1 * time.Second)

	// Запускаем игрока
	player := model.NewPlayer(multicastConn)
	player.Start()

	// Даем им пообщаться
	time.Sleep(30 * time.Second)
}
