package main

type App struct {
	source EventSource
}

func NewApp(source EventSource) *App {
	return &App{source: source}
}
