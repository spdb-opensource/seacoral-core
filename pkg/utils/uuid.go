package utils

import (
	"k8s.io/apimachinery/pkg/util/uuid"
)

func NewUUID() string {
	return string(uuid.NewUUID())
}
