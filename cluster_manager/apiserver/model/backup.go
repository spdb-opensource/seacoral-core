package model

import (
	sq "github.com/Masterminds/squirrel"
	"strings"
	"time"
)

const (
	// JobComplete means the job has completed its execution.
	// JobComplete JobConditionType = "Complete"
	// JobFailed means the job has failed its execution.
	// JobFailed JobConditionType = "Failed"

	BackupFileRunning      = "Running"
	BackupFileComplete     = "Complete"
	BackupFileFailed       = "Failed"
	BackupFileDeleting     = "Deleting"
	BackupFileDeleteFailed = "DeleteFailed"
)

type BackupStrategy struct {
	Active     bool        `db:"active"`
	Enabled    bool        `db:"enabled"`
	Retention  int         `db:"retention"`
	ID         string      `db:"id"`
	Name       string      `db:"name"`
	Desc       string      `db:"description"`
	App        string      `db:"app_id"`
	Unit       string      `db:"unit_id"`
	EndpointId string      `db:"endpoint_id"`
	Schedule   string      `db:"schedule"`
	Role       string      `db:"role"`
	Type       string      `db:"type"`
	Tables     SliceString `db:"tables"`

	Editor
}

func (BackupStrategy) Table() string {
	return "tbl_backup_strategy"
}

func (bs BackupStrategy) ObjectName() string {
	return bs.ID
}

type modelBackupStrategy struct {
	*dbBase
}

func (m modelBackupStrategy) InsertStrategy(bs BackupStrategy) (string, error) {
	query := "INSERT INTO " + bs.Table() +
		" (id,name,description,app_id,unit_id,endpoint_id,schedule,role,type,tables,retention,active,enabled,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:name,:description,:app_id,:unit_id,:endpoint_id,:schedule,:role,:type,:tables,:retention,:active,:enabled,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	if bs.ID == "" {
		bs.ID = newUUID(bs.Name)
	}

	_, err := m.NamedExec(query, bs)

	return bs.ID, err
}

func (m modelBackupStrategy) Lock(id string) (BackupStrategy, error) {
	bs := BackupStrategy{}

	err := m.txFrame(func(tx Tx) error {

		query := "UPDATE " + bs.Table() + " SET active=? WHERE id=? AND active<>?"

		_, err := tx.Exec(query, true, bs.ID, true)
		if err != nil {
			return err
		}

		query = "SELECT * FROM " + bs.Table() + " WHERE id=?"

		return tx.Get(&bs, query, id)
	})

	return bs, err
}

func (m modelBackupStrategy) UpdateStrategy(bs BackupStrategy) error {
	query := "UPDATE " + bs.Table() +
		" SET name=:name,app_id=:app_id,unit_id=:unit_id,endpoint_id=:endpoint_id,schedule=:schedule,role=:role,type=:type,tables=:tables,retention=:retention," +
		"active=:active,enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	_, err := m.NamedExec(query, bs)

	return err
}

func (m modelBackupStrategy) DeleteStrategy(id, app string) error {
	query := "DELETE FROM " + BackupStrategy{}.Table() + " WHERE id=? OR app_id=?"

	_, err := m.Exec(query, id, app)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m modelBackupStrategy) GetStrategy(id string) (BackupStrategy, error) {
	bs := BackupStrategy{}
	query := "SELECT * FROM " + bs.Table() + " WHERE id=?"

	err := m.dbBase.Get(&bs, query, id)

	return bs, err
}

func (m modelBackupStrategy) ListStrategy(selector map[string]string) ([]BackupStrategy, error) {
	if id, ok := selector["id"]; ok {

		bs, err := m.GetStrategy(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []BackupStrategy{bs}, err
	}

	var (
		err   error
		arg   interface{}
		list  []BackupStrategy
		query = "SELECT * FROM " + BackupStrategy{}.Table()
	)

	if name, ok := selector["name"]; ok {
		query += " WHERE name=?"
		arg = name

	} else if app, ok := selector["app_id"]; ok {
		query += " WHERE app_id=?"
		arg = app
	} else if unit, ok := selector["unit_id"]; ok {
		query += " WHERE unit_id=?"
		arg = unit
	} else if enabled, ok := selector[labelEnabled]; ok {
		query += " WHERE enabled=?"
		arg = enabled
	}

	if arg != nil {
		err = m.Select(&list, query, arg)
	} else {
		err = m.Select(&list, query)
	}

	return list, err
}

type BackupFile struct {
	Size        int64  `db:"size"`
	Status      string `db:"status"`
	ID          string `db:"id"`
	File        string `db:"file"`
	Type        string `db:"type"`
	EndpointId  string `db:"endpoint_id"`
	Site        string `db:"site_id"`
	Namespace   string `db:"namespace"`
	App         string `db:"app_id"`
	Unit        string `db:"unit_id"`
	Job         string `db:"job_name"`
	Task        string `db:"task_id"`
	Strategy    string `db:"strategy_id"`
	CreatedUser string `db:"created_user"`

	ExpiredAt  time.Time `db:"expired_timestamp"`
	CreatedAt  time.Time `db:"created_timestamp"`
	FinishedAt time.Time `db:"finished_timestamp"`
}

func (BackupFile) Table() string {
	return "tbl_backup_file"
}

func (bf BackupFile) ObjectName() string {
	return bf.Job
}

type modelBackupFile struct {
	*dbBase
}

func (m modelBackupFile) InsertFile(bf BackupFile) (string, error) {

	err := m.txFrame(func(tx Tx) error {

		tk := NewTask(ActionAppUnitRestore, bf.Unit, Unit{}.Table(), bf.CreatedUser)
		tk.ID, _ = m.txInsertTask(tx, tk)

		if bf.ID == "" {
			bf.ID = newUUID(bf.ID)
		}

		query := "INSERT INTO " + bf.Table() +
			" (id,file,type,endpoint_id,site_id,app_id,unit_id,strategy_id,task_id,namespace,job_name,created_user,size,status,expired_timestamp,created_timestamp,finished_timestamp) " +
			"VALUES (:id,:file,:type,:endpoint_id,:site_id,:app_id,:unit_id,:strategy_id,:task_id,:namespace,:job_name,:created_user,:size,:status,:expired_timestamp,:created_timestamp,:finished_timestamp)"

		bf.Task = tk.ID

		_, err := tx.NamedExec(query, bf)

		return err
	})

	return bf.ID, err
}

func (m modelBackupFile) UpdateFile(bf BackupFile) error {
	query := "UPDATE " + bf.Table() + " SET status=:status,expired_timestamp=:expired_timestamp,finished_timestamp=:finished_timestamp WHERE id=:id"

	_, err := m.NamedExec(query, bf)

	return err
}

func (m modelBackupFile) BackupJobDone(bf BackupFile) error {

	err := m.txFrame(func(tx Tx) error {
		query := "UPDATE " + bf.Table() + " SET size=:size, status=:status,expired_timestamp=:expired_timestamp,finished_timestamp=:finished_timestamp WHERE id=:id"

		_, err := tx.NamedExec(query, bf)
		if err != nil {
			return err
		}

		query = "UPDATE " + BackupStrategy{}.Table() + " SET active=? WHERE id=?"

		_, err = tx.Exec(query, false, bf.Strategy)
		if err != nil {
			return err
		}

		tk := Task{
			ID:         bf.Task,
			Status:     TaskSuccess,
			FinishedAt: time.Now(),
		}

		if bf.Status != BackupFileComplete {
			tk.Status = TaskFailed
		}

		err = m.txUpdateTask(tx, tk)

		return err
	})

	return err
}

func (m modelBackupFile) DeleteFile(id string) (err error) {
	query := "DELETE FROM " + BackupFile{}.Table() + " WHERE id=?"

	_, err = m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m modelBackupFile) GetFile(id string) (BackupFile, error) {
	bf := BackupFile{}
	query := "SELECT * FROM " + bf.Table() + " WHERE id=? OR job_name=?"

	err := m.dbBase.Get(&bf, query, id, id)

	return bf, err
}

func (m modelBackupFile) ListFiles(selector map[string]string) ([]BackupFile, error) {
	if id, ok := selector["id"]; ok {

		bf, err := m.GetFile(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []BackupFile{bf}, err
	}

	var (
		err   error
		arg   interface{}
		files []BackupFile
		query = "SELECT * FROM " + BackupFile{}.Table()
	)

	if app, ok := selector["app_id"]; ok {

		query += " WHERE app_id=?"
		arg = app

	} else if site, ok := selector["site_id"]; ok {

		query += " WHERE site_id=?"
		arg = site

	} else if status, ok := selector["status"]; ok {

		query += " WHERE status=?"
		arg = status

	} else if id, ok := selector["strategy_id"]; ok {

		query += " WHERE strategy_id=?"
		arg = id
	} else if createdUser, ok := selector["created_user"]; ok {

		query += " WHERE created_user=?"
		arg = createdUser
	}

	if arg != nil {
		err = m.Select(&files, query, arg)
	} else {
		err = m.Select(&files, query)
	}
	if err != nil {
		return nil, err
	}

	if expired, ok := selector["expired"]; ok {
		t, err := time.Parse("2006-01-02 15:04:05", expired)
		if err == nil {
			out := make([]BackupFile, 0, len(files))

			for i := range files {
				if t.After(files[i].ExpiredAt) {
					out = append(out, files[i])
				}
			}

			return out, nil
		}
	}

	return files, err
}

type fakeModelBackupStrategy struct{}

func (fakeModelBackupStrategy) InsertStrategy(bs BackupStrategy) (string, error) {
	return "", nil
}
func (m fakeModelBackupStrategy) Lock(id string) (BackupStrategy, error) {
	return BackupStrategy{}, nil
}
func (fakeModelBackupStrategy) UpdateStrategy(bs BackupStrategy) error {
	return nil
}
func (fakeModelBackupStrategy) DeleteStrategy(id, app string) error {
	return nil
}
func (fakeModelBackupStrategy) GetStrategy(id string) (BackupStrategy, error) {
	return BackupStrategy{}, nil
}
func (fakeModelBackupStrategy) ListStrategy(selector map[string]string) ([]BackupStrategy, error) {
	return nil, nil
}

type fakeModelBackupFile struct{}

func (fakeModelBackupFile) InsertFile(bf BackupFile) (string, error) {
	return "", nil
}
func (fakeModelBackupFile) BackupJobDone(bf BackupFile) error {
	return nil
}
func (fakeModelBackupFile) UpdateFile(bf BackupFile) error {
	return nil
}
func (fakeModelBackupFile) DeleteFile(id string) error {
	return nil
}
func (fakeModelBackupFile) GetFile(id string) (BackupFile, error) {
	return BackupFile{}, nil
}
func (fakeModelBackupFile) ListFiles(selector map[string]string) ([]BackupFile, error) {
	return nil, nil
}

type BackupEndpoint struct {
	ID      string `db:"id"`
	SiteId  string `db:"site_id"`
	Name    string `db:"name"`
	Type    string `db:"type"`
	Config  string `db:"endpoint_config"`
	Enabled bool   `db:"enabled"`
	Editor
}

func (BackupEndpoint) Table() string {
	return "tbl_backup_endpoint"
}

func (be BackupEndpoint) ObjectName() string {
	return be.ID
}

type modelBackupEndpoint struct {
	*dbBase
}

func (m modelBackupEndpoint) InsertEndpoint(be BackupEndpoint) (string, error) {
	if be.ID == "" {
		be.ID = newUUID("")
	}

	qh := "INSERT INTO " + be.Table() +
		" (id,site_id,name,type,endpoint_config,enabled,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:site_id,:name,:type,:endpoint_config,:enabled,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.NamedExec(qh, be)
		if err != nil {
			return err
		}

		return err
	})

	return be.ID, err
}

func (m modelBackupEndpoint) UpdateEndpoint(be BackupEndpoint) error {
	query := "UPDATE " + be.Table() + " SET site_id=:site_id," +
		"name=:name," +
		"type=:type, endpoint_config=:endpoint_config, enabled=:enabled," +
		"modified_user:=modified_user, modified_timestamp:=modified_timestamp " +
		" WHERE id=:id"

	_, err := m.NamedExec(query, be)

	return err
}

func (m modelBackupEndpoint) DeleteEndpoint(id string) (err error) {
	query := "DELETE FROM " + BackupEndpoint{}.Table() + " WHERE id=?"

	_, err = m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m modelBackupEndpoint) GetEndpoint(id string) (BackupEndpoint, error) {
	be := BackupEndpoint{}
	query := "SELECT * FROM " + be.Table() + " WHERE id=? OR name=?"

	err := m.dbBase.Get(&be, query, id, id)

	return be, err
}

func (m modelBackupEndpoint) ListEndpoint(selector map[string]string) ([]BackupEndpoint, error) {
	if id, ok := selector["id"]; ok {

		be, err := m.GetEndpoint(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []BackupEndpoint{be}, err
	}

	var (
		err           error
		backEndpoints []BackupEndpoint
		query         = sq.Select("*").From(BackupEndpoint{}.Table())
	)
	if app, ok := selector["site_id"]; ok {
		query = query.Where(sq.Eq{"site_id": strings.ToLower(app)})
	}
	if type_, ok := selector["type"]; ok {
		query = query.Where(sq.Eq{"type": strings.ToLower(type_)})
	}
	if name, ok := selector["name"]; ok {
		query = query.Where(sq.Eq{"name": strings.ToLower(name)})
	}

	sql, args, _ := query.ToSql()

	err = m.Select(&backEndpoints, sql, args...)
	if err != nil {
		return nil, err
	}

	return backEndpoints, err
}

type fakeModelBackupEndpoint struct{}

func (fakeModelBackupEndpoint) InsertEndpoint(be BackupEndpoint) (string, error) {
	return "", nil
}
func (fakeModelBackupEndpoint) UpdateEndpoint(be BackupEndpoint) error {
	return nil
}
func (fakeModelBackupEndpoint) DeleteEndpoint(id string) error {
	return nil
}
func (fakeModelBackupEndpoint) GetEndpoint(id string) (BackupEndpoint, error) {
	return BackupEndpoint{}, nil
}
func (fakeModelBackupEndpoint) ListEndpoint(selector map[string]string) ([]BackupEndpoint, error) {
	return []BackupEndpoint{}, nil
}
