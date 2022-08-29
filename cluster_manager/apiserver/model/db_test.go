package model

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var (
	db     = &dbBase{}
	testDB = false
)

// go test -v -args dbAddr=192.168.21.181:3306 dbDriver=mysql dbName=cluster_manager dbAuth=Y2x1c3Rlcl9tYW5hZ2VyOkNsdXN0ZXJNYW5hZ2VyQDEyMzQh dbMaxIdle=5 dbMaxOpen=10
func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	out, err := NewDBFromArgs(os.Args)
	if err != nil {
		fmt.Println(os.Args, err)
		return
	}

	db = out
	testDB = true
}

func randInt() int {
	return rand.Intn(100000)
}

func (db *dbBase) Get(dest interface{}, query string, args ...interface{}) error {
	if db == nil || db.DB == nil {
		return nil
	}

	return db.DB.Get(dest, query, args...)
}

func (db *dbBase) Select(dest interface{}, query string, args ...interface{}) error {
	if db == nil || db.DB == nil {
		return nil
	}

	return db.DB.Select(dest, query, args...)
}

func (db *dbBase) NamedExec(query string, arg interface{}) (sql.Result, error) {
	if db == nil || db.DB == nil {
		return nil, nil
	}

	return db.DB.NamedExec(query, arg)
}

func (db *dbBase) Exec(query string, args ...interface{}) (sql.Result, error) {
	if db == nil || db.DB == nil {
		return nil, nil
	}

	return db.DB.Exec(query, args...)
}

func (db *dbBase) Beginx() (fakeTx, error) {
	if db == nil || db.DB == nil {
		return fakeTxx{dbBase: db}, nil
	}

	txx, err := db.DB.Beginx()

	return fakeTxx{dbBase: db, tx: txx}, err
}

type fakeTxx struct {
	*dbBase
	tx *sqlx.Tx
}

func (txx fakeTxx) Commit() error {
	if txx.dbBase == nil || txx.dbBase.DB == nil || txx.tx == nil {
		return nil
	}

	return txx.tx.Commit()
}

func (txx fakeTxx) Rollback() error {
	if txx.dbBase == nil || txx.dbBase.DB == nil || txx.tx == nil {
		return nil
	}

	return txx.tx.Rollback()
}

type fakeTx interface {
	Tx
	Commit() error
	Rollback() error
}
