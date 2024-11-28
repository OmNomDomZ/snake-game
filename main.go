package main

import (
	"SnakeGame/ui"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("SnakeGame")
	myWindow.Resize(fyne.NewSize(1000, 800))
	myWindow.CenterOnScreen()

	ui.ShowMainMenu(myWindow)

	myWindow.ShowAndRun()
}
