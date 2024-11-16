package main

import (
	"SnakeGame/ui"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	a := app.New()
	w := a.NewWindow("Snake Game")
	w.Resize(fyne.NewSize(800, 600))

	// главное меню
	ui.ShowMainMenu(w)

	// запускаем
	w.ShowAndRun()
}
