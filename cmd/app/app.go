package main

import (
	"context"
)

// App holds the application state and exposes methods to the frontend.
type App struct {
	ctx context.Context
}

// startup is called when the app starts. The context is stored
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
