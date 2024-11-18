package main

import (
	"SnakeGame/connection"
	"SnakeGame/model"
	"time"
)

func main() {
	//a := app.New()
	//w := a.NewWindow("Snake Game")
	//w.Resize(fyne.NewSize(800, 600))
	//
	//// главное меню
	//ui.ShowMainMenu(w)
	//
	//// запускаем
	//w.ShowAndRun()

	multicastAddr, unicastAddr := connection.Connection()
	defer unicastAddr.Close()
	defer multicastAddr.Close()

	master := model.NewMaster(multicastAddr, unicastAddr)
	master.Start()

	time.Sleep(30 * time.Second)
}
