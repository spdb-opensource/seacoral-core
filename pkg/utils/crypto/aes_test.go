package crypto

import (
	"testing"

	"github.com/upmio/dbscale-kube/pkg/vars"
)

func TestExampleAesEncrypto(t *testing.T) {

	origin := "root"

	crypted := AesEncrypto(origin, vars.SeCretAESKey)
	t.Logf("pwd:%s,crypted:%s(len:%d)", origin, crypted, len(crypted))

	out, err := AesDecrypto(crypted, vars.SeCretAESKey)
	if err != nil {
		t.Fatalf("error:%v", err)
	}

	if origin != out {
		t.Logf("%s != %s", origin, out)
	}

}
