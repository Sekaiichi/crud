package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sekaiichi/crud/cmd/app"
	"github.com/sekaiichi/crud/pkg/customers"
	"github.com/sekaiichi/crud/pkg/managers"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/jackc/pgx/v4/stdlib"
	"go.uber.org/dig"
)

func main() {
	host := "0.0.0.0"
	port := "9999"
	dbConnectionString := "postgres://app:pass@localhost:5432/db"
	if err := execute(host, port, dbConnectionString); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func execute(host, port, dbConnectionString string) (err error) {
	dependencies := []interface{}{
		app.NewServer,
		mux.NewRouter,
		func() (*pgxpool.Pool, error) {
			connCtx, _ := context.WithTimeout(context.Background(), time.Second*5)
			return pgxpool.Connect(connCtx, dbConnectionString)
		},
		customers.NewService,
		managers.NewService,
		func(server *app.Server) *http.Server {
			return &http.Server{
				Addr:    host + ":" + port,
				Handler: server,
			}
		},
	}

	container := dig.New()
	for _, v := range dependencies {
		err = container.Provide(v)
		if err != nil {
			return err
		}
	}

	err = container.Invoke(func(server *app.Server) {
		server.Init()
	})
	if err != nil {
		return err
	}

	return container.Invoke(func(server *http.Server) error {
		return server.ListenAndServe()
	})
}
