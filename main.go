package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/c-bata/go-prompt"
	"github.com/k0kubun/pp"
	_ "github.com/mattn/go-oci8"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
	_ "golang.org/x/crypto/ssh/terminal"
	"io"
	"os/exec"
	"regexp"
	"strings"
	_ "syscall"

	"time"

	"os"
)

const (
	HistoryFileName = ".oracli_history"
)

func main() {
	app := cli.NewApp()
	app.Name = "oracli"
	app.Usage = "Yet Another Oracle CLI Client"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "hostname, H",
			Usage:  "Hostname",
			Value:  "localhost",
			EnvVar: "ORACLE_HOSTNAME",
		},
		cli.StringFlag{
			Name:   "username, u",
			Usage:  "Username",
			EnvVar: "ORACLE_USERNAME",
		},
		cli.StringFlag{
			Name:   "port, p",
			Usage:  "Port",
			Value:  "1521",
			EnvVar: "ORACLE_PORT",
		},
		cli.StringFlag{
			Name:   "service, s",
			Usage:  "Service",
			EnvVar: "ORACLE_SERVICE",
		},
		cli.StringFlag{
			Name:  "query, q",
			Usage: "Query",
		},
	}
	app.Action = func(context *cli.Context) error {
		hostname := context.String("hostname")
		username := context.String("username")
		service := context.String("service")
		port := context.Int("port")
		//fmt.Printf("Input your password: ")
		//password, err := terminal.ReadPassword(int(syscall.Stdin))
		//if err != nil {
		//	return err
		//}
		password := []byte("Oracle19")
		db, err := login(username, string(password), hostname, service, port)
		if err != nil {
			return err
		}
		defer func() {
			err := db.Close()
			if err != nil {
				fmt.Println("Close error is not nil:", err)
			}
		}()
		histories := readHistories()
		p := prompt.New(
			createExecutor(db),
			completer,
			prompt.OptionPrefix(">>> "),
			//prompt.OptionLivePrefix(changeLivePrefix),
			prompt.OptionTitle("live-prefix-example"),
			prompt.OptionHistory(histories),
		)
		p.Run()
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

func login(username, password, hostname, service string, port int) (*sql.DB, error) {
	// [username/[password]@]host[:port][/service_name][?param1=value1&...&paramN=valueN]
	openString := fmt.Sprintf("%s:%s@%s:%d/%s", username, password, hostname, port, service)

	db, err := sql.Open("oci8", openString)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("db is nil")
	}
	return db, nil
}

func query(db *sql.DB, q string) ([]string, [][]string, error) {
	var rows *sql.Rows
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	columns, _ := rows.Columns()
	count := len(columns)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)

	records := [][]string{}
	for rows.Next() {
		record := make([]string, count)
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, nil, err
		}

		for i, _ := range columns {
			record[i] = fmt.Sprintf("%v", values[i])
		}
		records = append(records, record)
	}

	err = rows.Err()
	if err != nil {
		return nil, nil, err
	}
	err = rows.Close()
	if err != nil {
		return nil, nil, err
	}
	return columns, records, nil
}

func createExecutor(db *sql.DB) func(string) {
	rCmd := regexp.MustCompile(`^:.*`)
	//rDesc := regexp.MustCompile(`^(?i)desc\s`)
	rSelect := regexp.MustCompile(`^(?i)select\s`)
	return func(in string) {
		//if in == "" {
		//	LivePrefixState.IsEnable = false
		//	LivePrefixState.LivePrefix = in
		//	return
		//}
		//LivePrefixState.LivePrefix = in + "> "
		//LivePrefixState.IsEnable = true
		if rCmd.MatchString(in) {
			cmdFields := strings.Fields(in[1:])
			cmd := exec.Command(cmdFields[0], cmdFields[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				fmt.Println(err.Error())
			}
			return
		}
		if rSelect.MatchString(in) {
			columns, records, err := query(db, in)
			printRecords(columns, records)
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	}
}

func printRecords(columns []string, records [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(columns)
	for _, record := range records {
		table.Append(record)
	}
	table.Render()
}

func completer(in prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "select", Description: "SELECT"},
		{Text: "insert", Description: "INSERT"},
		{Text: "delete", Description: "DELETE"},
		{Text: "update", Description: "UPDATE"},
		{Text: "desc", Description: "DESC"},
		{Text: "bash", Description: "Bash"},
	}
	return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
}

func readHistories() []string {
	fp, err := os.Open(HistoryFileName)
	if err != nil {
		return []string{}
	}
	defer fp.Close()
	histories := []string{}
	reader := bufio.NewReaderSize(fp, 4096)
	for {
		line, _, err := reader.ReadLine()
		histories = append(histories, string(line))
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
	}
	return histories
}

func debug(args ...interface{}) {
	pp.Println(args...)
}
