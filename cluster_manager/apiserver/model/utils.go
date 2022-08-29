package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/upmio/dbscale-kube/pkg/utils"
	"golang.org/x/xerrors"
)

func taskUUID(id string) string {
	return newUUID(id)
}

// func newUUID(prefix string) string {
// 	if len(prefix) > 16 {
// 		prefix = prefix[:16]
// 	}

// 	return prefix + "-" + utils.NewUUID()
// }

func newUUID(prefix string) string {
	return utils.NewUUID()
}

const (
	stringAndString = "&&"
	keyAndValue     = "=="
)

type MapString string

func NewMapString(m map[string]string) MapString {
	if len(m) == 0 {
		return ""
	}

	pairs := make([]string, 0, len(m))

	for key, val := range m {
		pairs = append(pairs, fmt.Sprintf("%s%s%s", key, keyAndValue, val))
	}

	return MapString(strings.Join(pairs, stringAndString))
}

func (ms MapString) Map() map[string]string {
	if ms == "" {
		return nil
	}

	parts := strings.Split(string(ms), stringAndString)

	m := make(map[string]string, len(parts))

	for i := range parts {

		kv := strings.SplitN(parts[i], keyAndValue, 2)
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}

	}

	return m
}

type SliceString string

func NewSliceString(s []string) SliceString {
	if len(s) == 0 {
		return ""
	}

	return SliceString(strings.Join(s, stringAndString))
}

func (ss SliceString) In(s string) bool {
	list := ss.Strings()

	for i := range list {
		if list[i] == s {
			return true
		}
	}

	return false
}

func (ss SliceString) Strings() []string {
	if len(ss) == 0 {
		return nil
	}

	return strings.Split(string(ss), stringAndString)
}

func IsNotExist(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(notFound); ok {
		return true
	}

	if xerrors.Is(err, sql.ErrNoRows) {
		return true
	}

	nf := notFound{}
	if xerrors.As(err, &nf) {
		return true
	}

	return strings.HasPrefix(err.Error(), "not found")
}

func NewNotFound(obj, name string) error {
	return notFound{
		object: obj,
		name:   name,
	}
}

type notFound struct {
	object string
	name   string
}

func (nf notFound) Error() string {
	return "not found " + nf.object + ":" + nf.name
}
