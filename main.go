package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"go-getting-started/internal/handler"
	"go-getting-started/internal/service"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/joho/godotenv"
	//"github.com/russross/blackfriday"
)

const (
	userDB     = "root"
	passwordDB = "root"
	//port       = 8080
)

var (
	port = env("PORT", "8080")
	//originStr = env("ORIGIN", "http://localhost:"+port)
	originStr = env("ORIGIN", "https://guarded-headland-35877.herokuapp.com")
	//databaseURL = env("DATABASE_URL", "root:root@/socialnet")
	databaseURL = env("JAWSDB_URL", "root:root@/socialnet")

	tokenKey = env("TOKEN_KEY", "supersecretkeyyoushouldnotcommit")

	// smtpHost     = env("SMTP_HOST", "smtp.mailtrap.io")
	// smtpPort     = env("SMTP_PORT", "25")
	// smtpUsername = mustEnv("SMTP_USERNAME")
	// smtpPassword = mustEnv("SMTP_PASSWORD")
)

func main() {
	godotenv.Load()
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {

	if databaseURL != "root:root@/socialnet" {
		databaseURL += `?useUnicode=true&characterEncoding=utf-8&reconnect=true`
	}

	//databaseURL = `xfmaig79aoxraj7y:j57liva88vtq9yqe@tcp(x8autxobia7sgh74.cbetxkdyhwsb.us-east-1.rds.amazonaws.com)/n3x1bykuec9jf1ua?useUnicode=true&characterEncoding=utf-8&reconnect=true`
	databaseURL = `xfmaig79aoxraj7y:j57liva88vtq9yqe@tcp(x8autxobia7sgh74.cbetxkdyhwsb.us-east-1.rds.amazonaws.com)/n3x1bykuec9jf1ua`
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	var useNats bool
	flag.StringVar(&allowedOrigin, "allowed-origin", allowedOrigin, "Allowed origin to do requests to this server. If empty, anyone will have access")
	flag.BoolVar(&useNats, "nats", false, "Whether use nats")
	flag.Parse()

	origin, err := url.Parse(originStr)
	if err != nil || !origin.IsAbs() {
		return errors.New("invalid origin url")
	}

	//db part
	db, err := sql.Open("mysql", databaseURL)
	if err != nil {
		panic(err)
	}

	// See "Important settings" section.
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	defer db.Close()

	if err = db.Ping(); err != nil {
		return fmt.Errorf("could not ping to db: %v", err)
	}
	log.Println("database opened successfully")
	service := service.New(
		db,
		//transport,
		//sender,
		*origin,
		tokenKey,
	)
	server := http.Server{
		Addr:              ":" + port,
		Handler:           handler.New(service, origin.Hostname() == "localhost"),
		ReadHeaderTimeout: time.Second * 5,
		ReadTimeout:       time.Second * 15,
	}

	errs := make(chan error, 2)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, os.Kill)

		<-quit

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			errs <- fmt.Errorf("could not shutdown server: %v", err)
			return
		}

		errs <- ctx.Err()
	}()

	go func() {
		log.Printf("accepting connections on port %s\n", port)
		log.Printf("starting server at %s\n", origin)
		if err = server.ListenAndServe(); err != http.ErrServerClosed {
			errs <- fmt.Errorf("could not listen and serve: %v", err)
			return
		}
		errs <- nil
	}()

	return <-errs
}

func env(key, fallbackValue string) string {
	s, ok := os.LookupEnv(key)
	if !ok {
		return fallbackValue
	}
	return s
}
