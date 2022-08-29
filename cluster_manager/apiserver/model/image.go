package model

import (
	"encoding/json"
	stderr "errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

// 镜像版本
type ImageVersion struct {
	ID string `db:"id"`
	// Registry string `db:"registry"`
	// 名称
	// example: mysql
	Type string `db:"type"`
	// arch
	Arch string `db:"arch"`
	// 主版本号
	Major int `db:"version_major"`
	// 小版本号
	Minor int `db:"version_minor"`
	// 小更新版本号
	Patch int `db:"version_patch"`
	// 编译版本号
	Dev int `db:"version_build"`
}

func (iv ImageVersion) Version() string {
	return fmt.Sprintf("%d.%d.%d.%d", iv.Major, iv.Minor, iv.Patch, iv.Dev)
}

func (iv ImageVersion) VersionWithArch() string {
	return fmt.Sprintf("%s-%s", iv.Version(), iv.Arch)
}

func (iv ImageVersion) ImageWithArch() string {
	return fmt.Sprintf("%s:%d.%d.%d.%d-%s", iv.Type, iv.Major, iv.Minor, iv.Patch, iv.Dev, iv.Arch)
}

func (iv ImageVersion) ImageTemplateFileName() string {
	return fmt.Sprintf("%s-%d.%d.%d.%d", iv.Type, iv.Major, iv.Minor, iv.Patch, iv.Dev)
}

func (iv ImageVersion) ImageTemplateFileNameWithArch() string {
	return fmt.Sprintf("%s-%s", iv.ImageTemplateFileName(), iv.Arch)
}

type Image struct {
	ImageVersion
	Unschedulable bool   `db:"unschedulable"`
	Desc          string `db:"description"`
	SiteID        string `db:"site_id"`
	ExporterPort  int    `db:"exporter_port"`

	KeySets        string `db:"key_sets"`
	ConfigTemplate string `db:"config_template"`
	PodTemplate    string `db:"pod_template"`

	Site SiteBrief `db:"-"`

	Editor
	Task Task `db:"-"`
}

func (im Image) ConvertToKeySets() ([]api.KeySet, error) {
	keySets := []api.KeySet{}
	err := json.Unmarshal([]byte(im.KeySets), &keySets)
	if err != nil {
		return []api.KeySet{}, err
	}

	return keySets, nil
}

func (im Image) ConvertToPodTemplate() (api.PodTemplate, error) {
	podTemplateData := api.PodTemplate{}
	err := json.Unmarshal([]byte(im.PodTemplate), &podTemplateData)
	if err != nil {
		return api.PodTemplate{}, err
	}

	return podTemplateData, nil
}

func (Image) Table() string {
	return "tbl_image"
}

func (iv Image) ObjectName() string {
	return fmt.Sprintf("%s-%d.%d.%d.%d", iv.Type, iv.Major, iv.Minor, iv.Patch, iv.Dev)
}

func NewImageModel() ModelImage {
	return &modelImage{}
}

type modelImage struct {
	*dbBase
}

// Insert insert Image and Task,returns Image.ID and Task.ID
func (m *modelImage) Insert(im Image) (string, string, error) {
	if im.ID == "" {
		//im.ID = newUUID(im.ImageVersion.String())
		im.ID = im.ImageVersion.ImageWithArch()
	}

	task := NewTask(ActionImageAdd, im.ID, im.Table(), im.CreatedUser)

	query := "INSERT INTO " + im.Table() +
		" (id,type,site_id,arch,version_major,version_minor,version_patch,version_build,unschedulable,description,exporter_port,key_sets,config_template,pod_template,created_timestamp,modified_timestamp) " +
		"VALUES (:id,:type,:site_id,:arch,:version_major,:version_minor,:version_patch,:version_build,:unschedulable,:description,:exporter_port,:key_sets,:config_template,:pod_template,:created_timestamp,:modified_timestamp)"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.NamedExec(query, im)
		if err != nil {
			return err
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return im.ID, task.ID, err
}

func (m *modelImage) InsertImageTask(im Image, action string) (string, error) {
	tk := NewTask(action, im.ID, im.Table(), im.CreatedUser)

	return m.insertTask(tk)
}

// Update set Image
func (m *modelImage) Update(im Image) error {
	query := "UPDATE " + im.Table() +
		" SET unschedulable=:unschedulable,description=:description,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	_, err := m.NamedExec(query, im)

	return err
}

func (m *modelImage) UpdateImageTask(im *Image, tk Task) error {
	if im == nil {
		return m.UpdateTask(tk)
	}

	return m.txFrame(func(tx Tx) error {

		query := "UPDATE " + im.Table() +
			" SET unschedulable=:unschedulable,description=:description,modified_timestamp=:modified_timestamp " +
			"WHERE id=:id"

		_, err := tx.NamedExec(query, im)
		if err != nil {
			return err
		}

		return m.txUpdateTask(tx, tk)
	})

}

func (m *modelImage) Delete(id string) error {
	query := "DELETE FROM " + Image{}.Table() + " WHERE id=?"

	_, err := m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m *modelImage) Get(id string) (Image, error) {
	im := Image{}
	pattern := ".*:[\\d]+\\.[\\d]+\\.[\\d]+\\.[\\d]+-(?:amd|arm)64"
	match, _ := regexp.Match(pattern, []byte(strings.ToLower(id)))
	if !match {
		return im, stderr.New("mysql must specify an valid image id and arch")
	}

	query := "SELECT * FROM " + im.Table() + " WHERE id=?"

	err := m.dbBase.Get(&im, query, id)
	if err != nil {
		return im, errors.Errorf("get image: %s from db ERR: %s", id, err)
	}

	im.Site, err = m.getSiteBrief(im.SiteID)

	im.Task, err = m.latestByRelateID(im.ID)

	return im, err
}

func (m *modelImage) GetLatest(type_, arch string) (Image, error) {
	im := Image{}

	query := "SELECT * FROM " + im.Table() + " WHERE type=? AND arch=? order by id desc limit 1"

	err := m.dbBase.Get(&im, query, type_, arch)
	if err != nil {
		return im, errors.Errorf("get latest image: %s %s from db ERR: %s", type_, arch, err)
	}

	im.Site, err = m.getSiteBrief(im.SiteID)

	im.Task, err = m.latestByRelateID(im.ID)

	return im, err
}

func (m *modelImage) List(selector map[string]string) ([]Image, error) {
	if id, ok := selector["id"]; ok {

		im, err := m.Get(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []Image{im}, err
	}

	images := []Image{}
	args := make([]interface{}, 0, 5)
	query := "SELECT * FROM " + Image{}.Table()

	if siteID, ok := selector["site_id"]; ok {
		query = query + " WHERE site_id=?"
		args = append(args, siteID)

		if v, ok := selector["major"]; ok && v != "" {
			query = query + " AND version_major=?"

			args = append(args, v)

			if v, ok := selector["minor"]; ok && v != "" {
				query = query + " AND version_minor=?"

				args = append(args, v)

				if v, ok := selector["patch"]; ok && v != "" {
					query = query + " AND version_patch=?"

					args = append(args, v)

					if v, ok := selector["build"]; ok && v != "" {
						query = query + " AND version_build=?"

						args = append(args, v)
					}
				}
			}
		}

		if unschedulable, ok := selector[labelUnschedulable]; ok {
			query = query + " AND unschedulable=?"
			args = append(args, unschedulable)
		}

		if t, ok := selector["type"]; ok {
			query = query + " AND type=?"
			args = append(args, t)
		}

	} else if v, ok := selector["type"]; ok && v != "" {
		query = query + " WHERE type=?"

		args = append(args, v)

		if unschedulable, ok := selector[labelUnschedulable]; ok {
			query = query + " AND unschedulable=?"
			args = append(args, unschedulable)
		}

		if v, ok := selector["major"]; ok && v != "" {
			query = query + " AND version_major=?"

			args = append(args, v)

			if v, ok := selector["minor"]; ok && v != "" {
				query = query + " AND version_minor=?"

				args = append(args, v)

				if v, ok := selector["patch"]; ok && v != "" {
					query = query + " AND version_patch=?"

					args = append(args, v)

					if v, ok := selector["build"]; ok && v != "" {
						query = query + " AND version_build=?"

						args = append(args, v)
					}
				}
			}
		}
	} else if unschedulable, ok := selector[labelUnschedulable]; ok {
		query = query + " WHERE unschedulable=?"
		args = append(args, unschedulable)

		if v, ok := selector["major"]; ok && v != "" {
			query = query + " AND version_major=?"

			args = append(args, v)

			if v, ok := selector["minor"]; ok && v != "" {
				query = query + " AND version_minor=?"

				args = append(args, v)

				if v, ok := selector["patch"]; ok && v != "" {
					query = query + " AND version_patch=?"

					args = append(args, v)

					if v, ok := selector["build"]; ok && v != "" {
						query = query + " AND version_build=?"

						args = append(args, v)
					}
				}
			}
		}
	} else if v, ok := selector["major"]; ok && v != "" {
		query = query + " WHERE version_major=?"

		args = append(args, v)

		if v, ok := selector["minor"]; ok && v != "" {
			query = query + " AND version_minor=?"

			args = append(args, v)

			if v, ok := selector["patch"]; ok && v != "" {
				query = query + " AND version_patch=?"

				args = append(args, v)

				if v, ok := selector["build"]; ok && v != "" {
					query = query + " AND version_build=?"

					args = append(args, v)
				}
			}
		}
	}

	err := m.Select(&images, query, args...)
	if err != nil {
		return images, err
	}

	tasks, err := m.tasksByTable(Image{}.Table())

	for i := range images {
		images[i].Site, _ = m.getSiteBrief(images[i].SiteID)
		for k := range tasks {
			if images[i].ID == tasks[k].RelateID &&
				images[i].Task.Auto < tasks[k].Auto {
				images[i].Task = tasks[k]
			}
		}
	}

	return images, nil
}

type fakeModelImage struct {
	images *sync.Map
	tasks  ModelTask
}

// Insert insert Image and Task,returns Image.ID and Task.ID
func (m *fakeModelImage) Insert(im Image) (string, string, error) {
	im.ID = newUUID(im.ImageVersion.ImageWithArch())

	task := NewTask(ActionImageAdd, im.ID, im.Table(), im.CreatedUser)

	m.images.Store(im.ID, im)
	task.ID, _ = m.tasks.Insert(task)

	return im.ID, task.ID, nil
}

func (m *fakeModelImage) InsertImageTask(_ Image, _ string) (string, error) {
	return "", nil
}

func (m *fakeModelImage) Update(im Image) error {

	m.images.Store(im.ID, im)

	return nil
}

func (m *fakeModelImage) UpdateImageTask(_ *Image, _ Task) error {
	return nil
}

func (m *fakeModelImage) Delete(id string) error {
	v, ok := m.images.Load(id)
	if !ok {
		return nil
	}

	m.images.Delete(v.(Image).ID)

	return nil
}

func (m *fakeModelImage) Get(id string) (Image, error) {
	v, ok := m.images.Load(id)
	if !ok {
		return Image{}, NewNotFound("image", id)
	}

	im := v.(Image)

	task, err := m.tasks.LatestByRelateID(im.ID)

	im.Task = task

	return im, err
}

func (m *fakeModelImage) GetLatest(type_, arch string) (Image, error) {
	return m.Get("notused")
}

func (m *fakeModelImage) List(selector map[string]string) ([]Image, error) {
	if id, ok := selector["id"]; ok {
		im, err := m.Get(id)

		return []Image{im}, err
	}

	images := make([]Image, 0, 1)

	if name, ok := selector["type"]; ok {

		m.images.Range(func(key, value interface{}) bool {
			v, ok := value.(Image)
			if ok && v.Type == name {
				images = append(images, v)
			}

			return true
		})

	} else if len(selector) == 0 {

		m.images.Range(func(key, value interface{}) bool {
			v, ok := value.(Image)
			if ok {
				images = append(images, v)
			}

			return true
		})
	}

	tasks, err := m.tasks.List(map[string]string{
		"latest": "latest",
		"table":  Image{}.Table(),
	})

	if err != nil {
		return images, err
	}

loop:
	for i := range images {
		for t := range tasks {
			if images[i].ID == tasks[t].RelateID {
				images[i].Task = tasks[t]

				continue loop
			}
		}
	}

	return images, nil
}
