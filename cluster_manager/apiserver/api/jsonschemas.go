package api

import (
	"github.com/gobuffalo/packr/v2"
)

var box *packr.Box

func init() {
	box = packr.New("My Box", "./jsonschemas")
}

func GetJsonSchema(schema string) ([]byte, error) {
	ret, err := box.Find(schema)
	if err != nil {
		return []byte{}, err
	} else {
		return ret, nil
	}
}
