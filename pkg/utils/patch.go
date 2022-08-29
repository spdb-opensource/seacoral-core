package utils

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func GenerateMergePatch(old interface{}, update interface{}, dataStruct interface{}) ([]byte, bool, error) {
	modJson, err := json.Marshal(update)
	if err != nil {
		return []byte{}, false, err
	}

	curJson, err := json.Marshal(old)
	if err != nil {
		return []byte{}, false, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, dataStruct)
	if err != nil {
		return []byte{}, false, fmt.Errorf("CreateTwoWayMergePatch fail:%s", err.Error())
	}

	if len(patch) == 0 || string(patch) == "{}" {
		return patch, false, nil
	}
	return patch, true, nil
}
