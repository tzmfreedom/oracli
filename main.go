package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/mattn/go-oci8"
	"github.com/urfave/cli"
	"log"
	"time"

	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "oracli"
	app.Usage = "Yet Another Oracle CLI Client"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "hostname, H",
			Usage: "Hostname",
		},
		cli.StringFlag{
			Name:  "username, u",
			Usage: "Username",
		},
		cli.StringFlag{
			Name:  "database, d",
			Usage: "Database",
		},
	}
	app.Action = func(context *cli.Context) error {
		//hostname := context.String("hostname")
		//username := context.String("username")
		rows, err := query("SELECT * FROM dual")
		if err != nil {
			return err
		}
		debug(rows)
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

func query(q string) (map[string]string, error) {
	var username, password, service string
	var port int

	// [username/[password]@]host[:port][/service_name][?param1=value1&...&paramN=valueN]
	openString := fmt.Sprintf("%s@%s:%d/%s", username, password, port, service)

	db, err := sql.Open("oci8", openString)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("db is nil")
	}

	// defer close database
	defer func() {
		err = db.Close()
		if err != nil {
			fmt.Println("Close error is not nil:", err)
		}
	}()

	var rows *sql.Rows
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	rows, err = db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	columns, _ := rows.Columns()
	count := len(columns)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)

	for rows.Next() {
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, err
		}

		for i, col := range columns {
			val := values[i]

			b, ok := val.([]byte)
			var v interface{}
			if ok {
				v = string(b)
			} else {
				v = val
			}

			fmt.Println(col, v)
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}
	err = rows.Close()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func debug(args ...interface{}) {
	pp.Println(args...)
}
