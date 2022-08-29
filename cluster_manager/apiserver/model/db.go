package model

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/pkg/utils/crypto"
)

type DBConfig struct {
	Driver       string
	Addr         string
	DBName       string
	Auth         string
	User         string
	Password     string
	MaxIdleConns int
	MaxOpenConns int
}

type dbBase struct {
	*sqlx.DB
}

func (db *dbBase) ModelCluster() ModelCluster {
	return &modelCluster{
		dbBase: db,
	}
}

func (db *dbBase) ModelSite() ModelSite {
	return &modelSite{
		dbBase: db,
	}
}

func (db *dbBase) ModelNetwork() ModelNetwork {
	return &modelNetwork{
		dbBase: db,
	}
}

func (db *dbBase) ModelHost() ModelHost {
	return &modelHost{
		dbBase: db,
	}
}

func (db *dbBase) ModelImage() ModelImage {
	return &modelImage{
		dbBase: db,
	}
}

func (db *dbBase) ModelTask() ModelTask {
	return &modelTask{
		dbBase: db,
	}
}

func (db *dbBase) ModelRemoteStorage() ModelRemoteStorage {
	return &modelRemoeteStorage{
		dbBase: db,
	}
}

func (db *dbBase) ModelApp() ModelApp {
	return &modelApp{
		dbBase: db,
	}
}

func (db *dbBase) ModelBackupStrategy() ModelBackupStrategy {
	return &modelBackupStrategy{
		dbBase: db,
	}
}

func (db *dbBase) ModelBackupFile() ModelBackupFile {
	return &modelBackupFile{
		dbBase: db,
	}
}

func (db *dbBase) ModelBackupEndpoint() ModelBackupEndpoint {
	return &modelBackupEndpoint{
		dbBase: db,
	}
}

// NewDB connect to a database and verify with Ping.
func NewDB(config DBConfig) (*dbBase, error) {
	if config.Auth != "" && config.User == "" {
		user, pw, err := crypto.Base64Decode(config.Auth)
		if err != nil {
			return nil, errors.Wrapf(err, "Base64:'%s'", config.Auth)
		}

		config.User = user
		config.Password = pw
	}

	source := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8&loc=Asia%%2FShanghai&sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION'",
		config.User, config.Password, config.Addr, config.DBName)

	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 2 * config.MaxIdleConns
	}

	db, err := sqlx.Connect(config.Driver, source)
	if err != nil {
		if db != nil {
			db.Close()
		}

		return nil, errors.Wrap(err, "DB connection")
	}

	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	return &dbBase{DB: db}, nil
}

// NewDBFromArgs args example:
// go test -v -args dbAddr=192.168.4.130:3306 dbDriver=mysql dbName=mgm  dbAuth=cm9vdDpyb290 dbTablePrefix=tbl dbMaxIdle=5 dbMaxOpen=10
func NewDBFromArgs(args []string) (*dbBase, error) {
	config := DBConfig{}

	for i := range args {

		list := strings.Split(args[i], " ")

		for l := range list {
			parts := strings.SplitN(list[l], "=", 2)
			if len(parts) != 2 {
				continue
			}

			val := strings.TrimSpace(parts[1])

			switch strings.TrimSpace(parts[0]) {
			case "dbAuth":
				config.Auth = val

			case "user":
				config.User = val

			case "password":
				config.Password = val

			case "dbDriver":
				config.Driver = val

			case "dbName":
				config.DBName = val

			case "dbAddr":
				config.Addr = val

			case "dbMaxIdle":
				if val == "" {
					val = "0"
				}
				config.MaxIdleConns, _ = strconv.Atoi(val)

			case "dbMaxOpen":
				if val == "" {
					val = "0"
				}
				config.MaxOpenConns, _ = strconv.Atoi(val)

			default:
			}
		}
	}

	if config.Driver == "" ||
		config.DBName == "" ||
		config.Addr == "" {
		return nil, errors.New("db config is required")
	}

	return NewDB(config)
}

func (db *dbBase) txFrame(do func(Tx) error) error {
	tx, err := db.Beginx()
	if err != nil {
		return errors.Wrap(err, "Tx Begin")
	}

	err = do(tx)
	if err == nil {
		return errors.Wrap(tx.Commit(), "Tx Commit")
	}

	if _err := tx.Rollback(); _err != nil {
		return fmt.Errorf("%s\n%+v", _err, err)
	}

	return err
}

type Tx interface {
	//	Commit() error
	//	Rollback() error

	Get(dest interface{}, query string, args ...interface{}) error
	// Select(dest interface{}, query string, args ...interface{}) error

	// Preparex(query string) (*sqlx.Stmt, error)
	// PrepareNamed(query string) (*sqlx.NamedStmt, error)

	Exec(query string, args ...interface{}) (sql.Result, error)
	NamedExec(query string, arg interface{}) (sql.Result, error)
}
