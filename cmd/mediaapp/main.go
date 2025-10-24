package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/GoArmGo/MediaApp/internal/di"
)

func main() {

	mode := flag.String("mode", "server", "Режим запуска приложения: server или worker")
	flag.Parse()

	ctx := context.Background()

	app, err := di.BuildApp()
	if err != nil {
		log.Fatalf("[main] ошибка инициализации приложения: %v", err)
	}

	if err := app.Run(ctx, mode); err != nil {
		log.Fatalf("[main] критическая ошибка во время выполнения: %v", err)
	}

	fmt.Println("[main] Приложение завершило работу корректно.")
}
