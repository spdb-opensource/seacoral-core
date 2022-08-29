package bankend

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/astaxie/beego/config"
	stderror "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"

	execapi "github.com/upmio/dbscale-kube/cluster_engine/plugin/execservice/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	"github.com/upmio/dbscale-kube/pkg/structs"
	"github.com/upmio/dbscale-kube/pkg/zone"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const baseDir = "/opt/upload_images/"

// lijj32(change log): baseDir was hardcoded as /opt/upload_images/,
// therefore this program could not run on windows platform,
// so I change it to a variable to allow users to specify the path with command line argument
// when starting CM-apiserver.
var baseImageDir = baseDir

func init() {
	flag.StringVar(&baseImageDir, "baseImageDir", baseDir, "base image dir")
}

func NewImageBankend(zone zone.ZoneInterface, sites siteGetter, m modelImage) *bankendImage {
	return &bankendImage{
		sites: sites,
		m:     m,
		zone:  zone,
		waits: NewWaitTasks(),
	}
}

type bankendImage struct {
	sites siteGetter
	m     modelImage

	zone zone.ZoneInterface

	waits *waitTasks
}

type modelImage interface {
	imageGetter

	Insert(model.Image) (string, string, error)
	InsertImageTask(model.Image, string) (string, error)
	Update(model.Image) error
	UpdateImageTask(*model.Image, model.Task) error
	Delete(name string) error
}

type imageGetter interface {
	Get(name string) (model.Image, error)
	GetLatest(type_, arch string) (model.Image, error)
	List(selector map[string]string) ([]model.Image, error)
}

func (b *bankendImage) Add(ctx context.Context, config api.ImageConfig) (api.Image, error) {
	site, err := b.sites.Get(config.Site)
	if err != nil {
		return api.Image{}, err
	}

	im, err := convertToModelImage(config)
	if err != nil {
		return api.Image{}, err
	}

	if err := checkImageTemplate(im); err != nil {
		return api.Image{}, err
	}

	s, err := b.zone.GetSite(site.ID)
	if err != nil {
		klog.Errorf("b.zone.GetSiteStr:%s err:%s", config.Site, err)
		return api.Image{}, err
	}

	iface, err := s.SiteInterface()
	if err != nil {
		return api.Image{}, err
	}

	deployOpts := execapi.DeployImageOption{
		ImageRegistry: site.ImageRegistry,
		ProjectName:   site.ProjectName,
		Type:          im.Type,
		Arch:          im.Arch,
		Version:       im.Version(),
	}

	err = iface.ImageDeployExec().DeployImage(deployOpts)
	klog.Infof("EXEC DeployImage...")
	if err != nil {
		klog.Errorf("import image err: %s.", err)
		klog.Infof("Fail to import image.")
		return api.Image{}, err
	}

	{
		templateConfig, err := generateTemplateConfig(im)
		if err != nil {
			klog.Errorf("ImageAdd: generate template config ERR: %s", err)
			return api.Image{}, err
		}

		_, err = iface.ConfigMaps().Create(metav1.NamespaceDefault, templateConfig)
		klog.Infof("ImageAdd: Creating... config template")
		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Errorf("ImageAdd: create -config-template err: %s", err)
			return api.Image{}, err
		}
		klog.Infof("ImageAdd: Create config template Done")
	}

	{
		scriptConfig, err := generateScriptConfig(im, "script")
		if err != nil {
			return api.Image{}, err
		}

		_, err = iface.ConfigMaps().Create(metav1.NamespaceDefault, scriptConfig)
		klog.Infof("ImageAdd: Creating... script")
		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Errorf("ImageAdd: create -script err: %s", err)
			return api.Image{}, err
		}
		klog.Infof("ImageAdd: Create script Done")
	}

	if im.Type == structs.MysqlServiceType {
		toolkitScript, err := generateScriptConfig(im, "toolkit-script")
		if err != nil {
			return api.Image{}, err
		}

		_, err = iface.ConfigMaps().Create(metav1.NamespaceDefault, toolkitScript)
		klog.Infof("ImageAdd: Creating... toolkitScript")
		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Errorf("ImageAdd: create -toolkitScript err: %s", err)
			return api.Image{}, err
		}
		klog.Infof("ImageAdd: Create toolkitScript Done")
	}

	//return api.Image{}, nil
	im.ID = im.ImageVersion.ImageWithArch()
	im.Site.Name = site.Name

	im.ExporterPort = 9104
	if strings.ToLower(im.Type) == "cmha" {
		im.ExporterPort = 0
	}

	_, _, err = b.m.Insert(im)
	if err != nil {
		return api.Image{}, err
	}

	//im.Task.ID = task
	klog.Infof("ImageAdd: import image success")
	return convertToImageAPI(im), nil
}

func (b *bankendImage) AddAsync(ctx context.Context, config api.ImageConfig) (api.Image, error) {
	site, err := b.sites.Get(config.Site)
	if err != nil {
		return api.Image{}, err
	}

	im, err := convertToModelImage(config)
	if err != nil {
		return api.Image{}, err
	}

	if err := checkImageTemplate(im); err != nil {
		return api.Image{}, err
	}

	id, task, err := b.m.Insert(im)
	if err != nil {
		return api.Image{}, err
	}

	im.ID = id
	im.Task.ID = task
	im.Site.Name = site.Name
	//version := im.ImageVersion.String()

	wt := b.waits.NewWaitTask(im.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)

		if err == nil {
			im.Unschedulable = config.Unschedulable

			return b.m.UpdateImageTask(&im, tk)
		}

		return b.m.UpdateImageTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*3, func() (bool, error) {

		s, err := b.zone.GetSite(site.ID)
		if err != nil {
			klog.Errorf("b.zone.GetSiteStr:%s err:%s", config.Site, err)
			return false, err
		}

		iface, err := s.SiteInterface()
		if err != nil {
			return false, err
		}

		{
			templateConfig, err := generateTemplateConfig(im)
			if err != nil {
				klog.Errorf("generate template config ERR: %s", err)
				return false, err
			}

			_, err = iface.ConfigMaps().Create(metav1.NamespaceDefault, templateConfig)
			klog.Infof("Creating... config template")
			if err != nil && !errors.IsAlreadyExists(err) {
				klog.Errorf("create -config-template err: %s", err)
				return false, err
			}
			klog.Infof("Create config template Done")
		}

		{
			scriptConfig, err := generateScriptConfig(im, "script")
			if err != nil {
				return false, err
			}

			_, err = iface.ConfigMaps().Create(metav1.NamespaceDefault, scriptConfig)
			klog.Infof("Creating... script")
			if err != nil && !errors.IsAlreadyExists(err) {
				klog.Errorf("create -script err: %s", err)
				return false, err
			}

			klog.Infof("Create script Done")
		}

		if im.Type == structs.MysqlServiceType {
			toolkitScript, err := generateScriptConfig(im, "toolkit-script")
			if err != nil {
				return false, err
			}

			_, err = iface.ConfigMaps().Create(metav1.NamespaceDefault, toolkitScript)
			klog.Infof("ImageAdd: Creating... toolkitScript")
			if err != nil && !errors.IsAlreadyExists(err) {
				klog.Errorf("ImageAdd: create -toolkitScript err: %s", err)
				return false, err
			}
			klog.Infof("ImageAdd: Create toolkitScript Done")
		}

		{
			deployOpts := execapi.DeployImageOption{
				ImageRegistry: site.ImageRegistry,
				ProjectName:   site.ProjectName,
				Type:          im.Type,
				Arch:          im.Arch,
				Version:       im.Version(),
			}

			err = iface.ImageDeployExec().DeployImage(deployOpts)
			klog.Infof("EXEC DeployImage")
			if err != nil {
				klog.Errorf("import image err: %s", err)
				return false, err
			}
			klog.Infof("import image done")
		}

		return true, nil
	})

	return convertToImageAPI(im), nil
}

func convertToModelImage(c api.ImageConfig) (model.Image, error) {
	imageVersion := model.ImageVersion(c.ImageVersion)
	fileName := imageVersion.ImageTemplateFileNameWithArch()

	it, err := getImageTemplateFromFile(baseImageDir, fileName)
	if err != nil {
		klog.Errorf("bankend/image:getImageTemplateFromFile err: %s", err)
		return model.Image{}, fmt.Errorf("getImageTemplateFromFile err: %s", err)
	}

	return model.Image{
		Unschedulable:  c.Unschedulable,
		ImageVersion:   imageVersion,
		Editor:         newCreateEditor(c.User),
		SiteID:         c.Site,
		KeySets:        it.KeySets,
		ConfigTemplate: it.ConfigTemplate,
		PodTemplate:    it.PodTemplate,
	}, nil
}

func checkImageTemplate(im model.Image) error {
	pt, err := im.ConvertToPodTemplate()
	if err != nil {
		klog.Errorf("PodTemplate Data check failed: %v", err)
		return err
	}

	err = pt.Valid()
	if err != nil {
		return err
	}

	_, err = im.ConvertToKeySets()
	if err != nil {
		klog.Errorf("KeySets Data check failed: %s", err)
		return err
	}

	return nil
}
func getImageTemplateFromFile(basedir string, filename string) (api.ImageTempate, error) {
	// lijj32: we should use filepath module instead of joining strings.
	// baseFileDir := basedir + filename
	baseFileDir := filepath.Join(basedir, filename)
	klog.Info(baseFileDir)

	//keyset
	keySetsPath := filepath.Join(baseFileDir, filename+".json")
	keySetByte, err := ioutil.ReadFile(keySetsPath)
	if err != nil {
		klog.Errorf("Read: %s Err: %s", filename+".json", err)
		return api.ImageTempate{}, err
	}

	//podtemplate
	podTemplatePath := filepath.Join(baseFileDir, filename+".tmpl")
	podtmplByte, err := ioutil.ReadFile(podTemplatePath)
	if err != nil {
		klog.Errorf("Read: %s Err: %s", filename+".tmpl", err)
		return api.ImageTempate{}, err
	}

	//cnf
	configTemplatePath := filepath.Join(baseFileDir, filename+".cnf")
	cnftmplByte, err := ioutil.ReadFile(configTemplatePath)
	if err != nil {
		klog.Errorf("Read %s Err: %s", filename+".cnf", err)
		return api.ImageTempate{}, err
	}

	imageTemplateData := api.ImageTempate{
		KeySets:        string(keySetByte),
		PodTemplate:    string(podtmplByte),
		ConfigTemplate: string(cnftmplByte),
	}

	return imageTemplateData, nil
}

func generateTemplateConfig(im model.Image) (*corev1.ConfigMap, error) {
	cfgpath, err := structs.GetDefaultConfigPath(im.Type)
	if err != nil {
		return nil, err
	}

	config := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              im.ImageTemplateFileNameWithArch() + "-config-template",
			Namespace:         metav1.NamespaceDefault,
			UID:               uuid.NewUUID(),
			ResourceVersion:   "",
			Generation:        0,
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels:            nil,
		},
		Data: map[string]string{
			unitv4.ConfigDataTab:     im.ConfigTemplate,
			unitv4.ConfigFilePathTab: cfgpath,
		},
	}

	return config, nil
}

func getScriptFromFile(basedir string, filename, scriptType string) (string, error) {
	// lijj32: should use filepath module instead of joining strings.
	// baseFileDir := basedir + filename
	baseFileDir := filepath.Join(basedir, filename)
	scriptFile := fmt.Sprintf("%s.%s", filename, scriptType)
	scriptPath := filepath.Join(baseFileDir, scriptFile)

	scriptByte, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		klog.Errorf("Read %s Err: %s", scriptFile, err)
		return "", err
	}

	return string(scriptByte), nil
}

func generateScriptConfig(im model.Image, scriptType string) (*corev1.ConfigMap, error) {
	fileName := im.ImageTemplateFileNameWithArch()
	script, err := getScriptFromFile(baseImageDir, fileName, scriptType)
	if err != nil {
		return nil, err
	}

	config := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              im.ImageTemplateFileNameWithArch() + "-" + scriptType,
			Namespace:         metav1.NamespaceDefault,
			UID:               uuid.NewUUID(),
			ResourceVersion:   "",
			Generation:        0,
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels:            nil,
		},
		Data: map[string]string{
			unitv4.ScriptDataTab: script,
		},
	}

	return config, nil
}

func selectorFromOption(opts api.ImageListOptions) map[string]string {
	selector := make(map[string]string)
	if opts.ID != nil {
		selector["id"] = *opts.ID
	}
	if opts.Type != nil {
		selector["type"] = *opts.Type
	}
	if opts.Major != nil {
		selector["major"] = *opts.Major
	}
	if opts.Minor != nil {
		selector["minor"] = *opts.Minor
	}
	if opts.Patch != nil {
		selector["patch"] = *opts.Patch
	}
	if opts.Dev != nil {
		selector["build"] = *opts.Dev
	}
	if opts.Unschedulable != nil {
		selector[labelUnschedulable] = boolValue(*opts.Unschedulable)
	}
	if opts.SiteId != nil {
		selector["site_id"] = *opts.SiteId
	}

	return selector
}

func (b *bankendImage) List(ctx context.Context, opts api.ImageListOptions) ([]api.Image, error) {
	selector := selectorFromOption(opts)

	list, err := b.m.List(selector)
	if err != nil {
		return nil, err
	}

	images := make([]api.Image, len(list))

	for i := range list {
		images[i] = convertToImageAPI(list[i])
	}

	return images, nil
}

func convertToImageAPI(im model.Image) api.Image {
	return api.Image{
		ImageVersion:  api.ImageVersion(im.ImageVersion),
		Desc:          im.Desc,
		Unschedulable: im.Unschedulable,
		Task:          convertToTaskBrief(im.Task),
		Created:       api.NewEditor("", im.CreatedAt),
		Modified:      api.NewEditor("", im.ModifiedAt),
		Site:          api.NewIDName(im.SiteID, im.Site.Name),
	}
}

func (b *bankendImage) Set(ctx context.Context, id string, opts api.ImageOptions) (api.Image, error) {
	image := api.Image{}
	im, err := b.m.Get(id)
	if err != nil {
		return image, err
	}

	im, ok := mergeImage(im, opts)
	if ok {
		err = b.m.Update(im)
	}

	return convertToImageAPI(im), err
}

func mergeImage(im model.Image, opts api.ImageOptions) (model.Image, bool) {
	update := false

	if opts.Desc != nil && im.Desc != *opts.Desc {
		im.Desc = *opts.Desc
		update = true
	}

	if opts.Unschedulable != nil && im.Unschedulable != *opts.Unschedulable {
		im.Unschedulable = *opts.Unschedulable
		update = true
	}

	im.ModifiedAt = time.Now()

	return im, update
}

func (b *bankendImage) Delete(ctx context.Context, id string) (api.TaskObjectResponse, error) {
	im, err := b.m.Get(id)
	if err != nil && model.IsNotExist(err) {
		return api.TaskObjectResponse{}, err
	}

	if im.ID != "" {
		id = im.ID
	}

	task, err := b.m.InsertImageTask(im, model.ActionImageDelete)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	b.waits.Delete(id)

	wt := NewWaitTask(time.Second*30, func(err error) error {
		if err == nil {
			err = b.m.Delete(id)
		}

		tk := taskUpdate(task, err)

		return b.m.UpdateImageTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*2, func() (bool, error) {

		site, err := b.zone.GetSite(im.SiteID)
		klog.Infof("get site:%s ", im.SiteID)
		if err != nil {
			klog.Errorf("GetSiteStr Err: %s", err)
			return false, err
		}

		iface, err := site.SiteInterface()
		if err != nil {
			return false, err
		}

		if err != nil && !errors.IsNotFound(err) {
			klog.Errorf("Delete %s-config-template Err: %s", im.ImageTemplateFileNameWithArch(), err)
			return false, fmt.Errorf("delete %s-config-template Err: %s", im.ImageTemplateFileNameWithArch(), err)
		}

		err = iface.ConfigMaps().Delete(metav1.NamespaceDefault, im.ImageTemplateFileNameWithArch()+"-script", metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			klog.Errorf("Delete %s-script Err: %s", im.ImageTemplateFileNameWithArch(), err)
			return false, fmt.Errorf("delete %s-script Err: %s", im.ImageTemplateFileNameWithArch(), err)
		}

		klog.Infof("DELETED image %s ", im.ID)
		return true, nil

	})

	return api.TaskObjectResponse{
		ObjectID:   im.ID,
		ObjectName: im.ImageWithArch(),
		TaskID:     task,
	}, nil
}

func (b *bankendImage) ListImageTemplates(ctx context.Context, imageID string) (api.ImageTemplate, error) {
	im, err := b.m.Get(imageID)
	if err != nil {
		return api.ImageTemplate{}, err
	}

	ks, err := im.ConvertToKeySets()
	if err != nil {
		return api.ImageTemplate{}, err
	}

	if ks == nil || len(ks) == 0 {
		return api.ImageTemplate{
			Image:    api.ImageVersion(im.ImageVersion),
			Template: api.ConfigTemplate{},
		}, nil
	}

	iface, err := b.zone.SiteInterface(im.SiteID)
	if err != nil {
		return api.ImageTemplate{}, err
	}

	cm, err := iface.ConfigMaps().Get(metav1.NamespaceDefault, im.ImageTemplateFileNameWithArch()+"-config-template")
	if err != nil {
		return api.ImageTemplate{}, err
	}

	content := cm.Data[unitv4.ConfigDataTab]

	configer, err := config.NewConfigData("ini", []byte(content))
	if err != nil {
		return api.ImageTemplate{}, err
	}

	keySets := []api.KeySet{}
	for _, keyset := range ks {
		value, exist := beegoConfigString(configer, keyset.Key)
		if !exist {
			continue
		}
		keyset.Value = value
		keySets = append(keySets, keyset)
	}

	tmpl := api.ImageTemplate{
		Image: api.ImageVersion(im.ImageVersion),
		Template: api.ConfigTemplate{
			ConfigFile: "",
			Keysets:    keySets,
		},
	}

	return tmpl, nil
}

func (b *bankendImage) SetImageTemplate(ctx context.Context, id string, kvs []api.KeySet) (api.TaskObjectResponse, error) {
	/*im, err := b.m.Get(id)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	zface := zoneIface{b.zone}

	tmpl, err := zface.imageTemplate(im)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	err = mergeKeysets(tmpl.Spec.Template.Keysets, kvs)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	options := make(map[string]string, len(kvs))

	for i := range kvs {
		options[kvs[i].Key] = kvs[i].Value
	}

	err = parser.SetConfigTemplate(&tmpl.Spec.Template, options)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	kvsData, err := json.Marshal(tmpl.Spec.Template.Keysets)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}
	task, err := b.m.InsertImageTask(im, model.ActionImageTemplateEdit)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	done := make(map[string]bool)
	_, file := filepath.Split(tmpl.Spec.Template.ConfigFile)
	kvsName := tmpl.Spec.Template.Image.ImageVersion() + ".json"

	wt := b.waits.NewWaitTask(im.ID, time.Second*10, func(err error) error {
		tk := taskUpdate(task, err)

		return b.m.UpdateImageTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute, func() (bool, error) {

		sites := zface.listSites()
		errs := make([]error, 0, len(sites))

		for i := range sites {

			if ok := done[sites[i].Name()]; ok {
				continue
			}

			iface, err := sites[i].SiteInterface()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			clone, err := iface.ImageTemplates().Get(tmpl.Name)
			if errors.IsNotFound(err) {

				clone, err = iface.ImageTemplates().Create(&tmpl)
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if ref := clone.Spec.ConfigMapRef; ref != nil {

				_, err = iface.ConfigMaps().Get(ref.Namespace, ref.Name)
				if err != nil {
					errs = append(errs, err)
					continue
				}
			}

			if ref := tmpl.Spec.Template.ConfigMapRef; ref != nil {

				cm, err := iface.ConfigMaps().Get(ref.Namespace, ref.Name)
				if errors.IsNotFound(err) {

					cm = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: ref.Name,
							Namespace: ref.Namespace,
						},
						Data: map[string]string{
							file:    tmpl.Spec.Template.Content,
							kvsName: string(kvsData),
						},
					}

					cm, err = iface.ConfigMaps().Create(ref.Namespace, cm)

				} else if err == nil {
					cm.Data[file] = tmpl.Spec.Template.Content
					cm.Data[kvsName] = string(kvsData)

					cm, err = iface.ConfigMaps().Update(cm.Namespace, cm)
				}
				if err != nil {
					errs = append(errs, err)
					continue
				}
			}

			if clone.Status.Phase == v1alpha3.ImageReady {
				done[sites[i].Name()] = true
			}
		}

		if len(done) == len(sites) {
			klog.Infof("edit image template %s ready", tmpl.Name)
			return true, nil
		}

		return false, utilerrors.NewAggregate(errs)
	})

	return api.TaskObjectResponse{
		ObjectID:   im.ID,
		ObjectName: im.Image(),
		TaskID:     task,
	}, nil*/

	return api.TaskObjectResponse{}, nil
}

/*
func mergeKeysets(kvs []v1alpha3.Keyset, news []api.Keyset) error {

loop:
	for _, kv := range news {

		for i := range kvs {
			if kvs[i].Key == kv.Key {
				kvs[i].Value = kv.Value
				kvs[i].CanSet = kv.CanSet
				kvs[i].MustRestart = kv.MustRestart
				kvs[i].Default = kv.Default
				kvs[i].Desc = kv.Desc
				kvs[i].Range = kv.Range

				continue loop
			}
		}

		return fmt.Errorf("not found keyset %s", kv.Key)
	}

	return nil
}
*/

func (b *bankendImage) ListImageScripts(ctx context.Context, imageID string) (api.ImageScripts, error) {
	im, err := b.m.Get(imageID)
	if err != nil {
		return api.ImageScripts{}, err
	}

	iface, err := b.zone.SiteInterface(im.SiteID)
	if err != nil {
		return api.ImageScripts{}, err
	}

	script, err := iface.ConfigMaps().Get(metav1.NamespaceDefault, im.ImageTemplateFileNameWithArch()+"-script")
	if err != nil {
		return api.ImageScripts{}, stderror.Errorf("get configmap by image id: %s ERR: %s", imageID, err)
	}

	if script == nil {
		return api.ImageScripts{}, fmt.Errorf("Image: %s doesn't have script configmap", imageID)
	}

	if script.Data[unitv4.ScriptDataTab] == "" {
		return api.ImageScripts{}, fmt.Errorf("image Script Configmap: %s doesn't have script date", im.ImageTemplateFileNameWithArch()+"-script")
	}

	content := script.Data[unitv4.ScriptDataTab]

	is := api.ImageScripts{
		Image:         api.ImageVersion(im.ImageVersion),
		ScriptContent: content,
	}

	return is, nil
}

func (b *bankendImage) SyncImageScripts(ctx context.Context, imageID string, scriptType string) error {
	im, err := b.m.Get(imageID)
	if err != nil {
		return err
	}

	iface, err := b.zone.SiteInterface(im.SiteID)
	if err != nil {
		return err
	}

	cmName := fmt.Sprintf("%s-%s", im.ImageTemplateFileNameWithArch(), scriptType)
	old, err := iface.ConfigMaps().Get(metav1.NamespaceDefault, cmName)
	if err != nil {
		return fmt.Errorf("get configmap:[%s] by image id:[%s] ERR: %s", cmName, imageID, err)
	}

	if old == nil {
		return fmt.Errorf("Image: %s doesn't have script configmap: %s", imageID, cmName)
	}

	fileName := im.ImageTemplateFileNameWithArch()
	script, err := getScriptFromFile(baseImageDir, fileName, scriptType)
	if err != nil {
		return err
	}

	new := old.DeepCopy()
	new.Data[unitv4.ScriptDataTab] = script

	_, err = iface.ConfigMaps().Update(metav1.NamespaceDefault, new)

	return err
}
