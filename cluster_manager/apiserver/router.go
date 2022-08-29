package main

import (
	"flag"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/backup"
	"github.com/upmio/dbscale-kube/pkg/server"
	"k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/bankend"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/app"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/host"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/image"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/network"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/site"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/storage"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/task"

	_ "github.com/go-sql-driver/mysql"
	"github.com/upmio/dbscale-kube/pkg/vars"
	"github.com/upmio/dbscale-kube/pkg/zone"
)

var (
	dbConfig = model.DBConfig{
		MaxIdleConns: 5,
		MaxOpenConns: 10,
		Driver:       "mysql",
	}

	execServicePort = "8800"
)

func initDBConfig() {
	flag.BoolVar(&fakeDB, "fake", false, "test mode,fake database")

	flag.StringVar(&dbConfig.Addr, "dbAddr", dbConfig.Addr, "database addr")
	flag.StringVar(&dbConfig.Auth, "dbAuth", dbConfig.Auth, "database auth,encode by base64")
	flag.StringVar(&dbConfig.User, "dbUser", dbConfig.User, "database user")
	flag.StringVar(&dbConfig.Password, "dbPassword", dbConfig.Password, "database password")
	flag.StringVar(&dbConfig.Driver, "dbDriver", dbConfig.Driver, "database driver type")
	flag.StringVar(&dbConfig.DBName, "dbName", dbConfig.DBName, "database name")
	flag.IntVar(&dbConfig.MaxOpenConns, "dbMaxOpenConns", dbConfig.MaxOpenConns, "database max open connects")
	flag.IntVar(&dbConfig.MaxIdleConns, "dbMaxIdleConns", dbConfig.MaxIdleConns, "database max idle connects")
}

//routers router.Adder, wsRouters handlerrouter.Adder
func initRouter(srv *server.Server) error {
	zone := zone.NewZone(8)

	fm := model.NewFakeModels()
	ms := fm.ModelSite()
	mt := fm.ModelTask()
	mi := fm.ModelImage()
	mn := fm.ModelNetwork()
	mh := fm.ModelHost()
	mc := fm.ModelCluster()
	mrs := fm.ModelRemoteStorage()
	mas := fm.ModelApp()
	mbs := fm.ModelBackupStrategy()
	mbf := fm.ModelBackupFile()
	mbe := fm.ModelBackupEndpoint()

	if !fakeDB {
		db, err := model.NewDB(dbConfig)
		if err != nil {
			return err
		}

		klog.Infof("DB:%s %s connected !", dbConfig.Driver, dbConfig.Addr)

		ms = db.ModelSite()
		mt = db.ModelTask()
		mi = db.ModelImage()
		mn = db.ModelNetwork()
		mh = db.ModelHost()
		mc = db.ModelCluster()
		mrs = db.ModelRemoteStorage()
		mas = db.ModelApp()
		mbs = db.ModelBackupStrategy()
		mbf = db.ModelBackupFile()
		mbe = db.ModelBackupEndpoint()
	}

	siteBknd := bankend.NewSiteBankend(execServicePort, zone, ms, mc, mrs, srv)
	err := siteBknd.RestoreSites()
	if err != nil {
		return err
	}

	bbknd := bankend.NewBackupBankend(zone, ms, mbs, mbf, mbe, mas)
	err = bbknd.CronStartAndRestore()
	if err != nil {
		return err
	}

	site.RegisterSiteRoute(siteBknd, srv)
	task.RegisterTaskRoute(bankend.NewTaskBankend(mt), srv)
	network.RegisterNetworkRoute(bankend.NewNetworkBankend(zone, mn, ms, mc), srv)
	image.RegisterImageRoute(bankend.NewImageBankend(zone, ms, mi), srv)
	host.RegisterHostRoute(bankend.NewHostBankend(zone, mh, mc, ms, mrs, vars.SeCretAESKey), srv)
	host.RegisterClusterRoute(bankend.NewClusterBankend(ms, mn, mc, mh), srv)
	storage.RegisterStorageRoute(bankend.NewStorageBankend(zone, mrs, ms, vars.SeCretAESKey), srv)

	app.RegisterAppRoute(bankend.NewAppBankend(zone, mas, mi, ms, mc, mn, mh, mbf, mbe, mrs, mrs), srv)

	backup.RegisterBackupRoute(bbknd, srv)

	err = siteBknd.InitDashboards()
	if err != nil {
		return err
	}

	return nil
}
