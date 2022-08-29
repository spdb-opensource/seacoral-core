package apiclient

import (
	"testing"
)

func TestNewImageAPI(t *testing.T) {
	if Client == nil {
		t.Skip("")
	}

	/*imageconfig := api.ImageConfig{
		Desc: "xxxxx",
		ImageVersion: api.ImageVersion{
			BackupType:  "upredis",
			Major: 2,
			Minor: 0,
			Patch: 0,
		},
		Enabled: true,
		User:    "xxxx",
	}
	postimageresp, err := Client.PostImage(Ctx, imageconfig)
	if err == nil {
		t.Log("PostImage success!")
		t.Log(postimageresp)
	} else {
		t.Error("PostImage err:", err)
	}*/

	/*var (
		imageoptsenabled = true
		imageoptsdesc    = "xxxxxxx"
	)
	imageopts := api.ImageOptions{
		Enabled: &imageoptsenabled,
		Desc:    &imageoptsdesc,
		User:    "xxx",
	}
	_, err = Client.UpdateImage(Ctx, "upredis", imageopts)
	if err == nil {
		t.Log("UpdateImage success!")
	} else {
		t.Error("UpdateImage err:", err)
	}*/

	_, err := Client.ListImages(Ctx, "upredis", "upredis")
	if err == nil {
		t.Log("ListImages success!")
	} else {
		t.Error("ListImages err:", err)
	}

	/*err = Client.DeleteImage(Ctx, "mysql")
	if err == nil {
		t.Log("DeleteImage success!")
	} else {
		t.Error("DeleteImage err:", err)
	}*/

	/*_, err = Client.ListImageTemplates(Ctx, "mysql")
	if err == nil {
		t.Log("ListImageTemplates success!")
	} else {
		t.Error("ListImageTemplates err:", err)
	}*/

	/*err = Client.UpdateImageTemplate(Ctx, "mysql", imageconfig)
	if err == nil {
		t.Log("UpdateImageTemplate success!")
	} else {
		t.Error("UpdateImageTemplate err:", err)
	}*/

}
