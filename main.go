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
	"github.com/mitchellh/go-homedir"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	HistoryFileName = ".oracli_history"
)

var mode int

const (
	ModeTable = iota
	ModeExpand
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
		password, err := getPassword()
		db, err := login(username, password, hostname, service, port)
		if err != nil {
			return err
		}
		defer func() {
			err := db.Close()
			if err != nil {
				fmt.Println("Close error is not nil:", err)
			}
		}()
		histories, err := readHistories()
		if err != nil {
			return err
		}
		home, err := homedir.Dir()
		if err != nil {
			panic(err) // TODO: impl
		}
		fp, err := os.OpenFile(filepath.Join(home, HistoryFileName), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			panic(err) // TODO: impl
		}
		defer fp.Close()
		p := prompt.New(
			createExecutor(db, fp),
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

func executeDDL(db *sql.DB, q string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, q)
	return err
}

func executeDML(db *sql.DB, q string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	result, err := db.ExecContext(ctx, q)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}

func describe(db *sql.DB, table string) ([]string, [][]string, error) {
	return query(db, fmt.Sprintf("SELECT * FROM xxx WHERE = %s", table))
}

func createExecutor(db *sql.DB, history *os.File) func(string) {
	rCmd := regexp.MustCompile(`^:.*`)
	rSelect := regexp.MustCompile(`^(?i)select\s`)
	rExecDML := regexp.MustCompile(`^(?i)(insert|update|delete|truncate)\s`)
	rCreate := regexp.MustCompile(`^(?i)create\s`)
	rDrop := regexp.MustCompile(`^(?i)drop\s`)
	rDesc := regexp.MustCompile(`^(?i)desc(ribe)?\s+(.+)`)
	rShellExec := regexp.MustCompile(`^(?i)execute\s+(.+)`)
	rExtra := regexp.MustCompile(`^(?i)\\(.)`)
	rExit := regexp.MustCompile(`^(?i)exit\s*$`)
	return func(in string) {
		if in != "" {
			historyAppend(in, history)
		}
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
				printError(err)
			}
			return
		}
		if rDesc.MatchString(in) {
			table := rDesc.FindStringSubmatch(in)[1]
			columns, records, err := describe(db, table)
			if err != nil {
				printError(err)
			}
			printRecords(columns, records)
			if err != nil {
				printError(err)
			}
			return
		}
		if rExecDML.MatchString(in) {
			rowsAffected, err := executeDML(db, in)
			if err != nil {
				printError(err)
			} else {
				fmt.Printf("RowsAffected: %d\n", rowsAffected)
			}
			return
		}
		if rCreate.MatchString(in) || rDrop.MatchString(in) {
			err := executeDDL(db, in)
			if err != nil {
				printError(err)
			} else {
				if rCreate.MatchString(in) {
					fmt.Println("Create: success")
				} else {
					fmt.Println("Drop: success")
				}
			}
			return
		}
		if rShellExec.MatchString(in) {
			file := rShellExec.FindStringSubmatch(in)[1]
			sql, err := ioutil.ReadFile(file)
			if err != nil {
				printError(err)
			}
			columns, records, err := query(db, string(sql))
			printRecords(columns, records)
			if err != nil {
				printError(err)
			}
			return
		}
		if rExtra.MatchString(in) {
			cmd := rExtra.FindStringSubmatch(in)[1]
			switch cmd {
			case "x":
				if mode == ModeTable {
					mode = ModeExpand
					fmt.Println("Change Mode: expand")
				} else {
					mode = ModeTable
					fmt.Println("Change Mode: table")
				}
			}
		}
		if rExit.MatchString(in) {
		}
	}
}

func printRecords(columns []string, records [][]string) {
	if mode == ModeTable {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(columns)
		for _, record := range records {
			table.Append(record)
		}
		table.Render()
	} else {
		for i, record := range records {
			fmt.Printf("*************************** %d. row ***************************\n", i)
			for j, column := range columns {
				fmt.Printf("%s: %s\n", column, record[j])
			}
		}
	}
}

func completer(in prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "select", Description: "SELECT"},
		{Text: "insert", Description: "INSERT"},
		{Text: "delete", Description: "DELETE"},
		{Text: "update", Description: "UPDATE"},
		{Text: "desc", Description: "DESC"},
		{Text: "out", Description: "OUT"},
		{Text: "execute", Description: "IN"},
		{Text: ":", Description: "COMMAND"},
	}
	return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
}

func readHistories() ([]string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return []string{}, nil
	}
	fp, err := os.Open(filepath.Join(home, HistoryFileName))
	if err != nil {
		return []string{}, nil
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
			return nil, err
		}
	}
	return histories, nil
}

func printError(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
}

func getPassword() (string, error) {
	if true {
		return "Oracle19", nil
	} else {
		fmt.Printf("Input your password: ")
		password, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", err
		}
		return string(password), nil
	}
}

func historyAppend(history string, fp *os.File) {
	fmt.Fprintln(fp, history)
}

func debug(args ...interface{}) {
	pp.Println(args...)
}
