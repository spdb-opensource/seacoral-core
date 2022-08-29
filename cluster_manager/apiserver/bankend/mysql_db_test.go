package bankend

import (
	"bytes"
	"testing"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

func TestDecodeJson(t *testing.T) {
	output := `
	[
  {
    "name": "abccd",
    "character_set": "utf8mb4"
  },
  {
    "name": "abcddddcd",
    "character_set": "utf8mb4"
  }
]

`
	schema := []api.Schema{}
	buf := bytes.NewBuffer([]byte(output))

	err := decodeJson(buf, &schema)
	if err != nil {
		t.Error(err)
	}
}
